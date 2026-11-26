package builtins

import (
	"testing"

	"github.com/andreimladin/dongle/internal/index"
	"github.com/andreimladin/dongle/internal/state"
	"github.com/andreimladin/dongle/internal/testutil"
)

func TestUpgradeOne(t *testing.T) {
	cases := []struct {
		name       string
		installed  string // installed version of deploy
		index      *plugin
		wantCode   int
		wantVer    string // installed version afterwards
		wantAz     bool
		wantStdout []string
		wantStderr []string
	}{
		{
			name: "newer available", installed: "1.0.0", index: &deploy,
			wantVer: "1.2.0", wantAz: true,
			wantStdout: []string{"Upgraded deploy 1.0.0 -> 1.2.0"},
			wantStderr: []string{"Upgrading deploy 1.0.0 -> 1.2.0..."},
		},
		{
			name: "already current", installed: "1.2.0", index: &deploy,
			wantVer: "1.2.0", wantStdout: []string{"deploy is already up to date (1.2.0)"},
		},
		{
			name: "installed is ahead", installed: "2.0.0", index: &deploy, wantCode: 1,
			wantVer: "2.0.0", wantStderr: []string{"error: installed deploy 2.0.0 is newer than the index (v1.2.0); not downgrading"},
		},
		{
			name: "not in the index", installed: "1.0.0", wantCode: 1,
			wantVer: "1.0.0", wantStderr: []string{"error: deploy is not in the index"},
		},
		{
			name: "unparseable index version", installed: "1.0.0", index: &plugin{name: "deploy", version: "latest"}, wantCode: 1,
			wantVer: "1.0.0", wantStderr: []string{`deploy: invalid semver "latest"`},
		},
		{
			name: "newer but incompatible", installed: "1.0.0", index: &plugin{name: "deploy", version: "2.0.0", host: ">=5.0.0"}, wantCode: 1,
			wantVer: "1.0.0", wantStderr: []string{"requires host >=5.0.0"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newEnv(t)
			if c.index != nil {
				cacheIndex(t, "1", *c.index)
			} else {
				cacheIndex(t, "1")
			}
			installed(t, state.Installed{Name: "deploy", ActiveVersion: c.installed})
			var code int
			stdout, stderr := testutil.Capture(t, func() { code = Upgrade(host, proto, "deploy", index.SyncAsk) })
			if code != c.wantCode {
				t.Fatalf("exit %d, want %d; stderr:\n%s", code, c.wantCode, stderr)
			}
			if got := loadState(t).Plugins["deploy"].ActiveVersion; got != c.wantVer {
				t.Errorf("installed version = %q, want %q", got, c.wantVer)
			}
			if got := len(f.calls) > 0; got != c.wantAz {
				t.Errorf("downloaded = %v, want %v", got, c.wantAz)
			}
			mustContain(t, "stdout", stdout, c.wantStdout...)
			mustContain(t, "stderr", stderr, c.wantStderr...)
		})
	}
}

func TestUpgradeAll(t *testing.T) {
	f := newEnv(t)
	cacheIndex(t, "1",
		plugin{name: "alpha", version: "1.1.0"},
		plugin{name: "bravo", version: "1.0.0"},
		plugin{name: "charlie", version: "1.0.0"},
	)
	installed(t,
		state.Installed{Name: "alpha", ActiveVersion: "1.0.0"},   // upgrade
		state.Installed{Name: "bravo", ActiveVersion: "1.0.0"},   // current
		state.Installed{Name: "charlie", ActiveVersion: "2.0.0"}, // ahead: skipped, not an error
		state.Installed{Name: "delta", ActiveVersion: "1.0.0"},   // not in index: skipped
	)
	var code int
	stdout, stderr := testutil.Capture(t, func() { code = Upgrade(host, proto, "", index.SyncAsk) })
	if code != 0 {
		t.Fatalf("exit %d; stderr:\n%s", code, stderr)
	}
	if len(f.calls) != 1 || flagValue(f.calls[0], "--name") != pkgName("alpha") {
		t.Fatalf("az calls = %v, want one download of alpha", f.calls)
	}
	st := loadState(t)
	for name, want := range map[string]string{"alpha": "1.1.0", "bravo": "1.0.0", "charlie": "2.0.0", "delta": "1.0.0"} {
		if got := st.Plugins[name].ActiveVersion; got != want {
			t.Errorf("%s = %s, want %s", name, got, want)
		}
	}
	mustContain(t, "stdout", stdout, "Upgraded alpha 1.0.0 -> 1.1.0", "bravo is already up to date")
	mustContain(t, "stderr", stderr,
		"warning: skipping installed charlie 2.0.0 is newer than the index",
		"warning: skipping delta is not in the index",
		"1 upgraded, 1 already up to date, 2 skipped, 0 failed")
	mustNotContain(t, "stderr", stderr, "error:")
}

func TestUpgradeAllReportsFailures(t *testing.T) {
	f := newEnv(t)
	cacheIndex(t, "1", plugin{name: "alpha", version: "1.1.0"}, plugin{name: "bravo", version: "1.1.0"})
	installed(t,
		state.Installed{Name: "alpha", ActiveVersion: "1.0.0"},
		state.Installed{Name: "bravo", ActiveVersion: "1.0.0"})
	f.failFor[pkgName("alpha")] = true

	var code int
	_, stderr := testutil.Capture(t, func() { code = Upgrade(host, proto, "", index.SyncAsk) })
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	// One failure doesn't stop the rest.
	if v := loadState(t).Plugins["bravo"].ActiveVersion; v != "1.1.0" {
		t.Fatalf("bravo = %s, want upgraded despite alpha failing", v)
	}
	mustContain(t, "stderr", stderr, "error: 1 upgraded, 0 already up to date, 0 skipped, 1 failed")
}

func TestUpgradePreconditions(t *testing.T) {
	t.Run("named plugin not installed", func(t *testing.T) {
		newEnv(t)
		var code int
		_, stderr := testutil.Capture(t, func() { code = Upgrade(host, proto, "deploy", index.SyncAsk) })
		if code != 1 {
			t.Fatalf("exit %d", code)
		}
		mustContain(t, "stderr", stderr, "deploy is not installed; use `dongle install deploy`")
	})
	t.Run("nothing installed", func(t *testing.T) {
		newEnv(t)
		var code int
		// No index cached and no az: exit 0 shows the feed was never consulted.
		_, stderr := testutil.Capture(t, func() { code = Upgrade(host, proto, "", index.SyncAsk) })
		if code != 0 {
			t.Fatalf("exit %d; stderr:\n%s", code, stderr)
		}
		mustContain(t, "stderr", stderr, "No plugins installed; nothing to upgrade.")
	})
	t.Run("index unavailable", func(t *testing.T) {
		newEnv(t)
		installed(t, state.Installed{Name: "deploy", ActiveVersion: "1.0.0"})
		if code := quiet(t, func() int { return Upgrade(host, proto, "", index.SyncAsk) }); code != 1 {
			t.Fatalf("exit %d", code)
		}
	})
	t.Run("unreadable state", func(t *testing.T) {
		newEnv(t)
		corruptState(t)
		if code := quiet(t, func() int { return Upgrade(host, proto, "", index.SyncAsk) }); code != 1 {
			t.Fatalf("exit %d", code)
		}
	})
}
