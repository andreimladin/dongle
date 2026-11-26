package builtins

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/andreimladin/dongle/internal/index"
	"github.com/andreimladin/dongle/internal/state"
	"github.com/andreimladin/dongle/internal/testutil"
)

var (
	alpha = plugin{name: "alpha", version: "v1.0.0", host: ">=0.1.0", protocol: "v1"}
	bravo = plugin{name: "bravo", version: "2.1.0", host: ">=0.1.0", protocol: "v1"}
)

// initialize runs Initialize with output captured.
func initialize(t *testing.T) (code int, stdout, stderr string) {
	t.Helper()
	stdout, stderr = testutil.Capture(t, func() { code = Initialize(host, proto) })
	return code, stdout, stderr
}

// assertNothingChanged checks a failed first run left no trace: no index
// cache, no plugins, flag unset — so the next run starts over cleanly.
func assertNothingChanged(t *testing.T) {
	t.Helper()
	if index.HasCache() {
		t.Error("a failed first run left an index cache behind")
	}
	st := loadState(t)
	if st.DefaultsBootstrapped {
		t.Error("a failed first run was recorded as initialized")
	}
	if len(st.Plugins) != 0 {
		t.Errorf("a failed first run left plugins in state: %v", st.Plugins)
	}
	if entries, _ := os.ReadDir(state.PluginsDir()); len(entries) != 0 {
		t.Errorf("a failed first run left plugin files behind: %v", entries)
	}
}

func TestInitializeFirstRun(t *testing.T) {
	f := newEnv(t)
	f.publishIndex(t, "2024.05.01", []string{"alpha", "bravo"}, alpha, bravo)

	code, stdout, stderr := initialize(t)
	if code != 0 {
		t.Fatalf("exit %d; stderr:\n%s", code, stderr)
	}
	if stdout != "" {
		t.Errorf("Initialize wrote to stdout: %q", stdout)
	}

	// The latest index was downloaded and the defaults installed from it.
	if got := flagValue(f.indexCalls[0], "--version"); len(f.indexCalls) != 1 || got != "*" {
		t.Fatalf("index calls = %v, want one download of the latest (*)", f.indexCalls)
	}
	if got, want := f.downloadedPackages(), []string{pkgName("alpha"), pkgName("bravo")}; !reflect.DeepEqual(got, want) {
		t.Fatalf("plugin downloads = %v, want %v", got, want)
	}
	st := loadState(t)
	for name, version := range map[string]string{"alpha": "1.0.0", "bravo": "2.1.0"} {
		inst := st.Plugins[name]
		if inst.ActiveVersion != version || inst.Entrypoint != "dongle-"+name {
			t.Errorf("%s in state = %+v, want version %s, entrypoint dongle-%s", name, inst, version, name)
		}
		if _, err := os.Stat(filepath.Join(state.PluginVersionDir(name, version), "dongle-"+name)); err != nil {
			t.Errorf("%s not placed: %v", name, err)
		}
	}

	// The index became the cache, and first run is recorded as done.
	if v, _ := index.CachedVersion(); v != "2024.05.01" {
		t.Errorf("cached index = %q, want 2024.05.01", v)
	}
	if !st.DefaultsBootstrapped {
		t.Error("first run not recorded")
	}
	mustContain(t, "stderr", stderr, "Downloaded plugin index 2024.05.01",
		"Installed alpha 1.0.0", "Installed bravo 2.1.0", "Initialization complete.")
	if entries, _ := os.ReadDir(filepath.Join(state.DataDir(), ".staging")); len(entries) != 0 {
		t.Errorf("staging left behind: %v", entries)
	}

	// Every later run is silent and never touches the feed.
	f.indexCalls, f.calls = nil, nil
	code, stdout, stderr = initialize(t)
	if code != 0 || stdout != "" || stderr != "" || len(f.indexCalls)+len(f.calls) != 0 {
		t.Fatalf("second run: exit %d, stdout %q, stderr %q, az calls %v %v", code, stdout, stderr, f.indexCalls, f.calls)
	}
}

func TestInitializeIsANoOp(t *testing.T) {
	t.Run("no index feed baked in (plain go build)", func(t *testing.T) {
		f := newEnv(t)
		index.SetDefaults("", "", "", indexPackage)
		f.publishIndex(t, "1", []string{"alpha"}, alpha)
		code, stdout, stderr := initialize(t)
		if code != 0 || stdout != "" || stderr != "" || len(f.indexCalls) != 0 {
			t.Fatalf("exit %d, stdout %q, stderr %q, index calls %v", code, stdout, stderr, f.indexCalls)
		}
	})
	t.Run("already initialized", func(t *testing.T) {
		f := newEnv(t)
		f.publishIndex(t, "1", []string{"alpha"}, alpha)
		if err := (&state.State{DefaultsBootstrapped: true}).Save(); err != nil {
			t.Fatal(err)
		}
		code, _, stderr := initialize(t)
		if code != 0 || stderr != "" || len(f.indexCalls) != 0 {
			t.Fatalf("exit %d, stderr %q, index calls %v", code, stderr, f.indexCalls)
		}
	})
}

func TestInitializeSkipsDefaultsThatCanNeverInstall(t *testing.T) {
	f := newEnv(t)
	tooNew := plugin{name: "toonew", version: "1.0.0", host: ">=9.0.0", protocol: "v1"}
	noBuild := plugin{name: "nobuild", version: "1.0.0", host: ">=0.1.0", protocol: "v1", noPlatform: true}
	f.publishIndex(t, "1", []string{"ghost", "toonew", "nobuild", "alpha"}, tooNew, noBuild, alpha)

	code, _, stderr := initialize(t)
	if code != 0 {
		t.Fatalf("exit %d; stderr:\n%s", code, stderr)
	}
	// Retrying wouldn't help these, so they're skipped, not fatal.
	mustContain(t, "stderr", stderr,
		"warning: skipping default plugin ghost:",
		"warning: skipping default plugin toonew: toonew requires host >=9.0.0",
		"warning: skipping default plugin nobuild: nobuild 1.0.0 has no build for",
		"Installed alpha 1.0.0", "Initialization complete.")
	if got := f.downloadedPackages(); !reflect.DeepEqual(got, []string{pkgName("alpha")}) {
		t.Fatalf("plugin downloads = %v, want only alpha", got)
	}
	if st := loadState(t); !st.DefaultsBootstrapped || len(st.Plugins) != 1 {
		t.Fatalf("state = %+v", st)
	}
}

func TestInitializeKeepsPluginsInstalledBeforeFirstRun(t *testing.T) {
	f := newEnv(t)
	f.publishIndex(t, "1", []string{"alpha", "bravo"}, alpha, bravo)
	installed(t, state.Installed{Name: "alpha", ActiveVersion: "0.9.0"})

	if code, _, stderr := initialize(t); code != 0 {
		t.Fatalf("exit %d; stderr:\n%s", code, stderr)
	}
	if got := f.downloadedPackages(); !reflect.DeepEqual(got, []string{pkgName("bravo")}) {
		t.Fatalf("plugin downloads = %v, want only bravo", got)
	}
	if v := loadState(t).Plugins["alpha"].ActiveVersion; v != "0.9.0" {
		t.Fatalf("alpha = %s, want the hand-installed 0.9.0 left alone", v)
	}
}

func TestInitializeWithoutDefaultsFile(t *testing.T) {
	f := newEnv(t)
	f.publishIndex(t, "1", nil, alpha) // an index from before defaults.yaml existed
	if code, _, stderr := initialize(t); code != 0 {
		t.Fatalf("exit %d; stderr:\n%s", code, stderr)
	}
	if len(f.calls) != 0 {
		t.Fatalf("installed plugins with no defaults listed: %v", f.calls)
	}
	if st := loadState(t); !st.DefaultsBootstrapped {
		t.Fatal("first run not recorded")
	}
	if !index.HasCache() {
		t.Fatal("index not cached")
	}
}

func TestInitializeFailuresChangeNothing(t *testing.T) {
	cases := []struct {
		name       string
		setup      func(t *testing.T, f *fakeAz)
		wantStderr []string
	}{
		{
			name:  "feed unreachable",
			setup: func(t *testing.T, f *fakeAz) {}, // nothing published
			wantStderr: []string{"error: couldn't reach the feed to initialize dongle",
				"cause: az download " + indexPackage + "@*", "ERROR: package " + indexPackage + " not found"},
		},
		{
			name:       "az not installed",
			setup:      func(t *testing.T, f *fakeAz) { f.publishIndex(t, "1", []string{"alpha"}, alpha); f.missing = true },
			wantStderr: []string{"couldn't reach the feed", "the Azure CLI is required"},
		},
		{
			name: "a default fails to download after another was installed",
			setup: func(t *testing.T, f *fakeAz) {
				f.publishIndex(t, "1", []string{"alpha", "bravo"}, alpha, bravo)
				f.failFor[pkgName("bravo")] = true
			},
			wantStderr: []string{"couldn't reach the feed", "az download " + pkgName("bravo"), "ERROR: unauthorized"},
		},
		{
			name: "invalid defaults file",
			setup: func(t *testing.T, f *fakeAz) {
				f.index = tarGz(t, map[string]string{"VERSION": "1", index.DefaultsFile: "default:\n  - alpha\n"})
			},
			wantStderr: []string{"error: reading the plugin index's default plugins: invalid defaults.yaml"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newEnv(t)
			c.setup(t, f)
			code, stdout, stderr := initialize(t)
			if code != 1 {
				t.Fatalf("exit %d, want 1; stderr:\n%s", code, stderr)
			}
			if stdout != "" {
				t.Errorf("stdout = %q", stdout)
			}
			mustContain(t, "stderr", stderr, append(c.wantStderr, "Nothing was changed; run dongle again to retry.")...)
			mustNotContain(t, "stderr", stderr, "Initialization complete.")
			assertNothingChanged(t)
			if entries, _ := os.ReadDir(filepath.Join(state.DataDir(), ".staging")); len(entries) != 0 {
				t.Errorf("staging left behind: %v", entries)
			}
		})
	}
}

func TestInitializeRetriesCleanlyAfterAFailure(t *testing.T) {
	f := newEnv(t)
	f.publishIndex(t, "1", []string{"alpha", "bravo"}, alpha, bravo)
	f.failFor[pkgName("bravo")] = true
	if code, _, _ := initialize(t); code != 1 {
		t.Fatalf("first attempt: exit %d, want 1", code)
	}

	delete(f.failFor, pkgName("bravo"))
	if code, _, stderr := initialize(t); code != 0 {
		t.Fatalf("retry: exit %d; stderr:\n%s", code, stderr)
	}
	st := loadState(t)
	if !st.DefaultsBootstrapped || len(st.Plugins) != 2 {
		t.Fatalf("after retry: %+v", st)
	}
}

func TestInitializeUnreadableState(t *testing.T) {
	f := newEnv(t)
	f.publishIndex(t, "1", []string{"alpha"}, alpha)
	corruptState(t)
	code, _, stderr := initialize(t)
	if code != 1 || len(f.indexCalls) != 0 {
		t.Fatalf("exit %d, index calls %v", code, f.indexCalls)
	}
	mustContain(t, "stderr", stderr, "error: reading state:")
}

// rollbackDefaults must only remove what this run placed: other versions
// of a plugin and other plugins are left alone.
func TestRollbackDefaultsRemovesOnlyWhatItWasGiven(t *testing.T) {
	newEnv(t)
	installed(t,
		state.Installed{Name: "alpha", ActiveVersion: "1.0.0"},
		state.Installed{Name: "keep", ActiveVersion: "1.0.0"})
	other := state.PluginVersionDir("alpha", "0.9.0")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}

	rollbackDefaults([]state.Installed{{Name: "alpha", ActiveVersion: "1.0.0"}})

	if _, err := os.Stat(state.PluginVersionDir("alpha", "1.0.0")); !errors.Is(err, os.ErrNotExist) {
		t.Error("rolled-back version still on disk")
	}
	if _, err := os.Stat(other); err != nil {
		t.Error("another version of the plugin was removed")
	}
	st := loadState(t)
	if _, ok := st.Plugins["alpha"]; ok {
		t.Error("rolled-back plugin still in state")
	}
	if _, ok := st.Plugins["keep"]; !ok {
		t.Error("an unrelated plugin was removed from state")
	}
	if _, err := os.Stat(state.PluginVersionDir("keep", "1.0.0")); err != nil {
		t.Error("an unrelated plugin's files were removed")
	}
}
