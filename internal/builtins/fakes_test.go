package builtins

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/andreimladin/dongle/internal/azcli/azclitest"
	"github.com/andreimladin/dongle/internal/index"
	"github.com/andreimladin/dongle/internal/state"
	"github.com/andreimladin/dongle/internal/testutil"
)

// indexPackage is the index feed package every test's build "bakes in"
// (see newEnv).
const indexPackage = "dongle-index"

// publishIndex puts an index archive on the fake feed: its VERSION, its
// defaults.yaml (omitted when defaults is nil) and a manifest per plugin.
func publishIndex(t *testing.T, f *azclitest.Feed, version string, defaults []string, plugins ...plugin) {
	t.Helper()
	files := map[string]string{"VERSION": version + "\n"}
	if defaults != nil {
		files[index.DefaultsFile] = "defaults:\n"
		for _, d := range defaults {
			files[index.DefaultsFile] += "  - " + d + "\n"
		}
	}
	for _, p := range plugins {
		files["plugins/"+p.name+".yaml"] = p.yaml()
	}
	f.Publish(version, testutil.TarGz(t, files))
}

// newEnv isolates the test (temp HOME so ~/.dongle is a temp dir, empty
// PATH) and installs a fake feed (azclitest.Feed) as the Azure CLI, with
// an index feed identity "baked in"; the feed has no index until the test
// publishes one (publishIndex).
func newEnv(t *testing.T) *azclitest.Feed {
	t.Helper()
	testutil.Isolate(t)
	f := azclitest.New(indexPackage)
	azclitest.Install(t, f)
	index.SetDefaults("acme", "", "plugins", indexPackage)
	t.Cleanup(func() { index.SetDefaults("", "", "", "") })
	return f
}

// plugin is a test manifest; render writes it as index YAML.
type plugin struct {
	name, version, host, protocol, project string
	noPlatform                             bool
}

func (p plugin) yaml() string {
	var b strings.Builder
	fmt.Fprintf(&b, "name: %s\nversion: %s\nshortDescription: The %s plugin\n", p.name, p.version, p.name)
	fmt.Fprintf(&b, "requires:\n  host: %q\n  protocol: %q\n", p.host, p.protocol)
	fmt.Fprintf(&b, "feed:\n  organization: acme\n  feed: plugins\n  project: %q\n", p.project)
	if !p.noPlatform {
		fmt.Fprintf(&b, "platforms:\n  - selector: {os: %s, arch: %s}\n    package: %s-%s-%s\n",
			runtime.GOOS, runtime.GOARCH, p.name, runtime.GOOS, runtime.GOARCH)
	}
	fmt.Fprintf(&b, "support:\n  documentation: https://docs.example/%s\n  channel: \"#%s\"\n", p.name, p.name)
	return b.String()
}

func pkgName(name string) string { return name + "-" + runtime.GOOS + "-" + runtime.GOARCH }

// cacheIndex writes an index cache (as a previous `dongle update` would
// have) that's fresh, so index.Prepare's default mode uses it without
// contacting the feed.
func cacheIndex(t *testing.T, version string, plugins ...plugin) {
	t.Helper()
	dir := filepath.Join(state.DataDir(), "index", "plugins")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, p := range plugins {
		if err := os.WriteFile(filepath.Join(dir, p.name+".yaml"), []byte(p.yaml()), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(state.DataDir(), "index.version"), []byte(version), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := index.MarkChecked(); err != nil {
		t.Fatal(err)
	}
}

// installed records plugins as installed in state, with a binary on disk.
func installed(t *testing.T, plugins ...state.Installed) {
	t.Helper()
	st, err := state.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range plugins {
		if p.Entrypoint == "" {
			p.Entrypoint = "dongle-" + p.Name
		}
		dir := state.PluginVersionDir(p.Name, p.ActiveVersion)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, p.Entrypoint), []byte("old"), 0o755); err != nil {
			t.Fatal(err)
		}
		st.Plugins[p.Name] = p
	}
	if err := st.Save(); err != nil {
		t.Fatal(err)
	}
}

func loadState(t *testing.T) *state.State {
	t.Helper()
	st, err := state.Load()
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func corruptState(t *testing.T) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(state.DataDir(), "state.json"), []byte("{bad"), 0o644); err != nil {
		t.Fatal(err)
	}
}

const host, proto = "1.0.0", "v1"

var deploy = plugin{name: "deploy", version: "v1.2.0", host: ">=0.1.0", protocol: "v1"}

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
