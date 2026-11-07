package builtins

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andreimladin/dongle/internal/compat"
	"github.com/andreimladin/dongle/internal/host"
	"github.com/andreimladin/dongle/internal/index"
	"github.com/andreimladin/dongle/internal/state"
	"github.com/andreimladin/dongle/internal/testutil"
)

func TestInstall(t *testing.T) {
	f := newEnv(t)
	cacheIndex(t, "2024.1", deploy)

	var code int
	stdout, stderr := testutil.Capture(t, func() { code = Install(hostVer, proto, "deploy", index.SyncAsk) })
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

	// The downloaded file is placed under the canonical entrypoint name,
	// executable — <host.Name>-<plugin>, not named after the running binary
	// (here the test binary, builtins.test).
	bin := filepath.Join(state.PluginVersionDir("deploy", "1.2.0"), host.Name+"-deploy")
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
	want := state.Installed{Name: "deploy", ActiveVersion: "1.2.0", Entrypoint: host.Name + "-deploy",
		Requires: compat.Requires{Host: ">=0.1.0", Protocol: "v1"}}
	if got != want {
		t.Fatalf("state = %+v, want %+v", got, want)
	}

	mustContain(t, "stdout", stdout, "Installed deploy 1.2.0", "(run it with: "+host.Name+" deploy)")
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
	if code := quiet(t, func() int { return Install(hostVer, proto, "deploy", index.SyncAsk) }); code != 0 {
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
		{"no cached index", false, "deploy is already installed (version 1.0.0). To update it, run: " + host.Name + " upgrade deploy"},
		{"newer in cached index", true, "deploy is already installed (1.0.0). A newer version (1.2.0) is available — run: " + host.Name + " upgrade deploy"},
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
			_, stderr := testutil.Capture(t, func() { code = Install(hostVer, proto, "deploy", index.SyncAsk) })
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
			wantStderr: []string{"error: no plugin named nope in the index (see `" + host.Name + " search`)"},
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
			stdout, stderr := testutil.Capture(t, func() { code = Install(hostVer, proto, name, index.SyncAsk) })
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
