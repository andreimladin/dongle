package builtins

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andreimladin/dongle/internal/bootstrap"
	"github.com/andreimladin/dongle/internal/compat"
	"github.com/andreimladin/dongle/internal/index"
	"github.com/andreimladin/dongle/internal/state"
	"github.com/andreimladin/dongle/internal/testutil"
)

const host, proto = "1.0.0", "v1"

var deploy = plugin{name: "deploy", version: "v1.2.0", host: ">=0.1.0", protocol: "v1"}

// --- install ----------------------------------------------------------------

func TestInstall(t *testing.T) {
	f := newEnv(t)
	cacheIndex(t, "2024.1", deploy)

	var code int
	stdout, stderr := testutil.Capture(t, func() { code = Install(host, proto, "deploy", index.SyncAsk) })
	if code != 0 {
		t.Fatalf("exit %d; stderr:\n%s", code, stderr)
	}

	// The right package was requested from the plugin's own feed.
	if len(f.calls) != 1 {
		t.Fatalf("az calls = %v", f.calls)
	}
	args := strings.Join(f.calls[0], " ")
	mustContain(t, "az args", args, "artifacts universal download", "--organization https://dev.azure.com/acme",
		"--feed plugins", "--name "+pkgName("deploy"), "--version 1.2.0 ") // upack versions are bare semver
	mustNotContain(t, "az args", args, "--project")

	// The downloaded file is placed under the canonical entrypoint name, executable.
	bin := filepath.Join(state.PluginVersionDir("deploy", "1.2.0"), "dongle-deploy")
	fi, err := os.Stat(bin)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm()&0o111 == 0 {
		t.Fatalf("entrypoint not executable: %v", fi.Mode())
	}
	if b, _ := os.ReadFile(bin); !strings.Contains(string(b), pkgName("deploy")) {
		t.Fatalf("entrypoint is not the downloaded file: %q", b)
	}

	// State records what dispatch needs.
	got := loadState(t).Plugins["deploy"]
	want := state.Installed{Name: "deploy", ActiveVersion: "1.2.0", Entrypoint: "dongle-deploy",
		Requires: compat.Requires{Host: ">=0.1.0", Protocol: "v1"}}
	if got != want {
		t.Fatalf("state = %+v, want %+v", got, want)
	}

	mustContain(t, "stdout", stdout, "Installed deploy 1.2.0", "(run it with: dongle deploy)")
	mustContain(t, "stderr", stderr, "Installing deploy 1.2.0...")
	if entries, _ := os.ReadDir(filepath.Join(state.DataDir(), ".staging")); len(entries) != 0 {
		t.Fatalf("staging left behind: %v", entries)
	}
}

func TestInstallProjectScopedFeed(t *testing.T) {
	f := newEnv(t)
	p := deploy
	p.project = "tools"
	cacheIndex(t, "2024.1", p)
	if code := quiet(t, func() int { return Install(host, proto, "deploy", index.SyncAsk) }); code != 0 {
		t.Fatalf("exit %d", code)
	}
	mustContain(t, "az args", strings.Join(f.calls[0], " "), "--project tools --scope project")
}

func TestInstallAlreadyInstalled(t *testing.T) {
	cases := []struct {
		name    string
		index   bool // a cached index lists a newer version
		wantMsg string
	}{
		{"no cached index", false, "deploy is already installed (version 1.0.0). To update it, run: dongle upgrade deploy"},
		{"newer in cached index", true, "deploy is already installed (1.0.0). A newer version (1.2.0) is available — run: dongle upgrade deploy"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newEnv(t)
			if c.index {
				cacheIndex(t, "2024.1", deploy)
			}
			installed(t, state.Installed{Name: "deploy", ActiveVersion: "1.0.0"})
			var code int
			// No cached index and no az on PATH: if Install went on to
			// prepare the index it would fail, so exit 0 also proves it
			// stopped before touching the feed.
			_, stderr := testutil.Capture(t, func() { code = Install(host, proto, "deploy", index.SyncAsk) })
			if code != 0 {
				t.Fatalf("exit %d; stderr:\n%s", code, stderr)
			}
			mustContain(t, "stderr", stderr, c.wantMsg)
			if len(f.calls) != 0 || loadState(t).Plugins["deploy"].ActiveVersion != "1.0.0" {
				t.Fatal("an installed plugin must be left alone")
			}
		})
	}
}

func TestInstallFailures(t *testing.T) {
	cases := []struct {
		name       string
		setup      func(t *testing.T, f *fakeAz)
		plugin     string
		wantStderr []string
		wantAz     bool // whether a download should have been attempted
	}{
		{
			name:       "not in the index",
			setup:      func(t *testing.T, f *fakeAz) { cacheIndex(t, "1", deploy) },
			plugin:     "nope",
			wantStderr: []string{"error: no plugin named nope in the index (see `dongle search`)"},
		},
		{
			name: "host too old",
			setup: func(t *testing.T, f *fakeAz) {
				p := deploy
				p.host = ">=9.0.0"
				cacheIndex(t, "1", p)
			},
			wantStderr: []string{"deploy requires host >=9.0.0 but this host is 1.0.0 — not installing"},
		},
		{
			name: "protocol mismatch",
			setup: func(t *testing.T, f *fakeAz) {
				p := deploy
				p.protocol = "v2"
				cacheIndex(t, "1", p)
			},
			wantStderr: []string{`speaks protocol "v2" but this host supports "v1" — not installing`},
		},
		{
			name: "bad constraint",
			setup: func(t *testing.T, f *fakeAz) {
				p := deploy
				p.host = ">=soon"
				cacheIndex(t, "1", p)
			},
			wantStderr: []string{"bad constraint in manifest"},
		},
		{
			name: "no build for this platform",
			setup: func(t *testing.T, f *fakeAz) {
				p := deploy
				p.noPlatform = true
				cacheIndex(t, "1", p)
			},
			wantStderr: []string{"deploy v1.2.0 has no build for"},
		},
		{
			name:       "invalid manifest",
			setup:      func(t *testing.T, f *fakeAz) { cacheIndex(t, "1"); writeIndexFile(t, "deploy", "name: [") },
			wantStderr: []string{"invalid manifest for deploy"},
		},
		{
			name:       "download fails",
			setup:      func(t *testing.T, f *fakeAz) { cacheIndex(t, "1", deploy); f.failFor[pkgName("deploy")] = true },
			wantStderr: []string{"az download " + pkgName("deploy") + "@1.2.0", "ERROR: unauthorized"},
			wantAz:     true,
		},
		{
			name:       "package with several files",
			setup:      func(t *testing.T, f *fakeAz) { cacheIndex(t, "1", deploy); f.extra = true },
			wantStderr: []string{"contains 2 files, expected exactly 1"},
			wantAz:     true,
		},
		{
			name:       "az not installed",
			setup:      func(t *testing.T, f *fakeAz) { cacheIndex(t, "1", deploy); f.missing = true },
			wantStderr: []string{"the Azure CLI is required"},
		},
		{
			name:       "no index and no feed",
			setup:      func(t *testing.T, f *fakeAz) {},
			wantStderr: []string{"error:", "Azure CLI is required"},
		},
		{
			name:       "unreadable state",
			setup:      func(t *testing.T, f *fakeAz) { cacheIndex(t, "1", deploy); corruptState(t) },
			wantStderr: []string{"error:"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newEnv(t)
			c.setup(t, f)
			name := c.plugin
			if name == "" {
				name = "deploy"
			}
			var code int
			stdout, stderr := testutil.Capture(t, func() { code = Install(host, proto, name, index.SyncAsk) })
			if code != 1 {
				t.Fatalf("exit %d, want 1", code)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want nothing on failure", stdout)
			}
			mustContain(t, "stderr", stderr, c.wantStderr...)
			if got := len(f.calls) > 0; got != c.wantAz {
				t.Errorf("download attempted = %v, want %v", got, c.wantAz)
			}
			if st, err := state.Load(); err == nil {
				if _, ok := st.Plugins["deploy"]; ok {
					t.Error("a failed install was recorded in state")
				}
			}
			if _, err := os.Stat(filepath.Join(state.PluginsDir(), "deploy")); !os.IsNotExist(err) {
				t.Error("a failed install left a plugin dir behind")
			}
			if entries, _ := os.ReadDir(filepath.Join(state.DataDir(), ".staging")); len(entries) != 0 {
				t.Errorf("staging left behind: %v", entries)
			}
		})
	}
}

func writeIndexFile(t *testing.T, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(state.DataDir(), "index", "plugins", name+".yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// quiet runs fn with output captured and discarded, returning its code.
func quiet(t *testing.T, fn func() int) int {
	t.Helper()
	var code int
	testutil.Capture(t, func() { code = fn() })
	return code
}

func TestHostBinaryName(t *testing.T) {
	orig := os.Args[0]
	defer func() { os.Args[0] = orig }()
	cases := map[string]string{
		"/opt/bin/acme":          "acme",
		"dongle":                 "dongle",
		"./tools/acme-cli":       "acme-cli",
		"":                       "dongle",
		string(os.PathSeparator): "dongle",
	}
	for arg0, want := range cases {
		os.Args[0] = arg0
		if got := hostBinaryName(); got != want {
			t.Errorf("hostBinaryName() with argv[0] %q = %q, want %q", arg0, got, want)
		}
	}
}

func TestCopyFile(t *testing.T) {
	dir := t.TempDir()
	src, dst := filepath.Join(dir, "src"), filepath.Join(dir, "dst")
	os.WriteFile(src, []byte("payload"), 0o600)
	os.WriteFile(dst, []byte("previous, longer contents"), 0o644)
	if err := copyFile(src, dst, 0o755); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(dst)
	fi, _ := os.Stat(dst)
	if string(b) != "payload" || fi.Mode().Perm() != 0o755 {
		t.Fatalf("dst = %q mode %v", b, fi.Mode())
	}
	if err := copyFile(filepath.Join(dir, "missing"), dst, 0o755); err == nil {
		t.Fatal("want error for a missing source")
	}
	if err := copyFile(src, filepath.Join(dir, "no", "such", "dir"), 0o755); err == nil {
		t.Fatal("want error for an unwritable destination")
	}
}

// --- remove -----------------------------------------------------------------

func TestRemove(t *testing.T) {
	newEnv(t)
	installed(t,
		state.Installed{Name: "deploy", ActiveVersion: "1.0.0"},
		state.Installed{Name: "other", ActiveVersion: "2.0.0"})
	// A second version on disk goes too.
	os.MkdirAll(state.PluginVersionDir("deploy", "0.9.0"), 0o755)

	var code int
	stdout, _ := testutil.Capture(t, func() { code = Remove("deploy") })
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if _, err := os.Stat(filepath.Join(state.PluginsDir(), "deploy")); !os.IsNotExist(err) {
		t.Fatal("plugin dir not removed")
	}
	st := loadState(t)
	if _, ok := st.Plugins["deploy"]; ok {
		t.Fatal("still in state")
	}
	if _, ok := st.Plugins["other"]; !ok {
		t.Fatal("removed the wrong plugin from state")
	}
	if _, err := os.Stat(state.PluginVersionDir("other", "2.0.0")); err != nil {
		t.Fatal("other plugin's files were touched")
	}
	mustContain(t, "stdout", stdout, "Removed deploy")
}

func TestRemoveFailures(t *testing.T) {
	newEnv(t)
	var code int
	_, stderr := testutil.Capture(t, func() { code = Remove("deploy") })
	if code != 1 {
		t.Fatalf("exit %d", code)
	}
	mustContain(t, "stderr", stderr, "error: deploy is not installed")

	corruptState(t)
	if code := quiet(t, func() int { return Remove("deploy") }); code != 1 {
		t.Fatalf("unreadable state: exit %d", code)
	}
}

// --- upgrade ----------------------------------------------------------------

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

// --- search / support ---------------------------------------------------------

func TestSearch(t *testing.T) {
	newEnv(t)
	cacheIndex(t, "1",
		plugin{name: "zeta", version: "v0.1.0"},
		plugin{name: "alpha", version: "2.0.0"})
	var code int
	stdout, _ := testutil.Capture(t, func() { code = Search(index.SyncAsk) })
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	want := []string{"alpha 2.0.0 The alpha plugin", "zeta 0.1.0 The zeta plugin"}
	if len(lines) != len(want) {
		t.Fatalf("stdout:\n%s", stdout)
	}
	for i := range want {
		if got := strings.Join(strings.Fields(lines[i]), " "); got != want[i] {
			t.Errorf("line %d = %q, want %q", i, got, want[i])
		}
	}
}

func TestSearchEdgeCases(t *testing.T) {
	t.Run("empty index", func(t *testing.T) {
		newEnv(t)
		cacheIndex(t, "1")
		var code int
		stdout, stderr := testutil.Capture(t, func() { code = Search(index.SyncAsk) })
		if code != 0 || stdout != "" {
			t.Fatalf("exit %d stdout %q", code, stdout)
		}
		mustContain(t, "stderr", stderr, "The plugin index is empty.")
	})
	t.Run("no index available", func(t *testing.T) {
		newEnv(t)
		if code := quiet(t, func() int { return Search(index.SyncAsk) }); code != 1 {
			t.Fatalf("exit %d", code)
		}
	})
	t.Run("cache without a plugins dir", func(t *testing.T) {
		newEnv(t)
		cacheIndex(t, "1")
		os.RemoveAll(filepath.Join(state.DataDir(), "index", "plugins"))
		if code := quiet(t, func() int { return Search(index.SyncNever) }); code != 1 {
			t.Fatalf("exit %d", code)
		}
	})
}

func TestSupport(t *testing.T) {
	newEnv(t)
	cacheIndex(t, "1", deploy)
	// Appending a contact to the rendered manifest keeps it valid YAML.
	writeIndexFile(t, "withcontact", strings.Replace(deploy.yaml(), "name: deploy", "name: withcontact", 1)+"  contact: ops@example.com\n")

	var code int
	stdout, _ := testutil.Capture(t, func() { code = Support("deploy") })
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	mustContain(t, "stdout", stdout, "deploy — support", "Documentation:", "https://docs.example/deploy", "Channel:", "#deploy")
	mustNotContain(t, "stdout", stdout, "Contact:")

	stdout, _ = testutil.Capture(t, func() { code = Support("withcontact") })
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	mustContain(t, "stdout", stdout, "Contact:", "ops@example.com")
}

func TestSupportFailures(t *testing.T) {
	newEnv(t)
	if code := quiet(t, func() int { return Support("deploy") }); code != 1 {
		t.Fatalf("no index: exit %d", code)
	}
	cacheIndex(t, "1")
	var code int
	_, stderr := testutil.Capture(t, func() { code = Support("deploy") })
	if code != 1 {
		t.Fatalf("exit %d", code)
	}
	mustContain(t, "stderr", stderr, "no plugin named deploy in the index")
	writeIndexFile(t, "deploy", "name: [")
	if code := quiet(t, func() int { return Support("deploy") }); code != 1 {
		t.Fatalf("invalid manifest: exit %d", code)
	}
}

// --- update -------------------------------------------------------------------

func TestUpdateFailureIsExitOne(t *testing.T) {
	newEnv(t) // no az on PATH: the feed check fails
	var code int
	_, stderr := testutil.Capture(t, func() { code = Update() })
	if code != 1 {
		t.Fatalf("exit %d", code)
	}
	mustContain(t, "stderr", stderr, "could not update the plugin index")
}

// --- version ------------------------------------------------------------------

func TestVersion(t *testing.T) {
	t.Run("fresh install", func(t *testing.T) {
		newEnv(t)
		var code int
		stdout, _ := testutil.Capture(t, func() { code = Version("1.4.0") })
		if code != 0 {
			t.Fatalf("exit %d", code)
		}
		lines := fieldsByLine(stdout)
		want := []string{"dongle 1.4.0", "index none (run `dongle update`)", "", "no plugins installed"}
		if strings.Join(lines, "\n") != strings.Join(want, "\n") {
			t.Fatalf("got:\n%s", stdout)
		}
	})
	t.Run("embedded index and plugins", func(t *testing.T) {
		newEnv(t)
		cacheIndex(t, "2024.03.01")
		os.WriteFile(filepath.Join(state.DataDir(), "index.origin"), []byte(index.OriginEmbedded), 0o644)
		installed(t,
			state.Installed{Name: "zeta", ActiveVersion: "0.1.0"},
			state.Installed{Name: "alpha", ActiveVersion: "1.0.0"})
		stdout, _ := testutil.Capture(t, func() { Version("dev") })
		want := []string{"dongle dev", "index 2024.03.01 (embedded)", "", "installed plugins:", "alpha 1.0.0", "zeta 0.1.0"}
		if got := fieldsByLine(stdout); strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Fatalf("got:\n%s", stdout)
		}
	})
	t.Run("fetched index has no note", func(t *testing.T) {
		newEnv(t)
		cacheIndex(t, "2024.03.01")
		stdout, _ := testutil.Capture(t, func() { Version("dev") })
		if fieldsByLine(stdout)[1] != "index 2024.03.01" {
			t.Fatalf("got:\n%s", stdout)
		}
	})
	t.Run("unreadable state", func(t *testing.T) {
		newEnv(t)
		corruptState(t)
		var code int
		_, stderr := testutil.Capture(t, func() { code = Version("dev") })
		if code != 1 {
			t.Fatalf("exit %d", code)
		}
		mustContain(t, "stderr", stderr, "error: reading state:")
	})
}

// fieldsByLine normalizes column padding: each line's fields joined by a
// single space.
func fieldsByLine(s string) []string {
	var out []string
	for _, l := range strings.Split(strings.TrimRight(s, "\n"), "\n") {
		out = append(out, strings.Join(strings.Fields(l), " "))
	}
	return out
}

// --- initialize -----------------------------------------------------------------

func TestInitialize(t *testing.T) {
	d1 := bootstrap.Default{Name: "deploy", Version: "1.0.0"}
	d2 := bootstrap.Default{Name: "lint", Version: "0.3.0"}
	cases := []struct {
		name       string
		cached     bool
		payload    fakePayload
		wantSeed   bool
		wantInst   []string
		wantMark   bool
		wantStderr []string
		notStderr  []string
	}{
		{
			name:      "plain build does nothing",
			notStderr: []string{"Initialization complete."},
		},
		{
			name:      "already initialized does nothing",
			cached:    true,
			payload:   fakePayload{hasIndex: true},
			notStderr: []string{"Initialization complete.", "Initializing plugin index"},
		},
		{
			name:       "first run seeds the index and installs defaults",
			payload:    fakePayload{hasIndex: true, defaults: []bootstrap.Default{d1, d2}},
			wantSeed:   true,
			wantInst:   []string{"deploy", "lint"},
			wantMark:   true,
			wantStderr: []string{"Initialized plugin index", "Installed deploy 1.0.0", "Installed lint 0.3.0", "Initialization complete."},
		},
		{
			name:       "defaults only",
			cached:     true,
			payload:    fakePayload{hasIndex: true, defaults: []bootstrap.Default{d1}},
			wantInst:   []string{"deploy"},
			wantMark:   true,
			wantStderr: []string{"Installed deploy 1.0.0", "Initialization complete."},
			notStderr:  []string{"Initialized plugin index"},
		},
		{
			name:       "failures are warnings and the rest still runs",
			payload:    fakePayload{hasIndex: true, seedErr: errors.New("disk full"), defaults: []bootstrap.Default{d1, d2}, installErr: map[string]error{"deploy": errors.New("boom")}, markErr: errors.New("ro fs")},
			wantSeed:   true,
			wantInst:   []string{"lint"},
			wantMark:   true,
			wantStderr: []string{"warning: could not initialize the embedded plugin index: disk full", "warning: installing embedded default deploy: boom", "Installed lint 0.3.0", "warning: could not record embedded defaults bootstrap: ro fs", "Initialization complete."},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			newEnv(t)
			if c.cached {
				cacheIndex(t, "1")
			}
			p := c.payload
			orig := payload
			payload = &p
			defer func() { payload = orig }()

			stdout, stderr := testutil.Capture(t, Initialize)
			if stdout != "" {
				t.Errorf("Initialize wrote to stdout: %q", stdout)
			}
			if p.seeded != c.wantSeed {
				t.Errorf("seeded = %v, want %v", p.seeded, c.wantSeed)
			}
			if strings.Join(p.installedAs, ",") != strings.Join(c.wantInst, ",") {
				t.Errorf("installed %v, want %v", p.installedAs, c.wantInst)
			}
			if p.marked != c.wantMark {
				t.Errorf("marked = %v, want %v", p.marked, c.wantMark)
			}
			mustContain(t, "stderr", stderr, c.wantStderr...)
			mustNotContain(t, "stderr", stderr, c.notStderr...)
		})
	}
}
