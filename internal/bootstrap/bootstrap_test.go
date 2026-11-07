//go:build embed

package bootstrap

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"testing/fstest"

	"github.com/andreimladin/dongle/internal/compat"
	"github.com/andreimladin/dongle/internal/state"
	"github.com/andreimladin/dongle/internal/testutil"
)

// Run with: go test -tags embed ./internal/bootstrap

func withPayload(t *testing.T, files fstest.MapFS) {
	t.Helper()
	orig := payloadFS
	payloadFS = files
	t.Cleanup(func() { payloadFS = orig })
}

const manifestJSON = `{
  "plugins": [
    {"name": "deploy", "version": "1.0.0", "file": "dongle-deploy", "requires": {"host": ">=0.1.0", "protocol": "v1"}},
    {"name": "lint", "version": "0.2.0", "file": "bin/lint", "entrypoint": "custom-lint"}
  ],
  "index": {"version": "2024.03.01"}
}`

func TestPendingDefaults(t *testing.T) {
	testutil.Isolate(t)

	withPayload(t, fstest.MapFS{})
	if d := PendingDefaults(); d != nil {
		t.Fatalf("nothing staged: got %v", d)
	}

	withPayload(t, fstest.MapFS{"embedded/manifest.json": {Data: []byte("{bad")}})
	var d []Default
	_, stderr := testutil.Capture(t, func() { d = PendingDefaults() })
	if d != nil || stderr == "" {
		t.Fatalf("invalid manifest: got %v, stderr %q", d, stderr)
	}

	withPayload(t, fstest.MapFS{"embedded/manifest.json": {Data: []byte(manifestJSON)}})
	d = PendingDefaults()
	if len(d) != 2 || d[0].Name != "deploy" || d[1].Entrypoint != "custom-lint" {
		t.Fatalf("got %+v", d)
	}
	if err := MarkDefaultsBootstrapped(); err != nil {
		t.Fatal(err)
	}
	if d := PendingDefaults(); d != nil {
		t.Fatalf("after bootstrap: got %v, want none", d)
	}
}

func TestInstallDefault(t *testing.T) {
	testutil.Isolate(t)

	// Each default is declared once; the fake payload and the expected
	// on-disk results are derived from it, so renaming a plugin here is
	// a one-place change.
	cases := []struct {
		def       Default
		payload   string // the staged file's contents
		wantEntry string // the installed file's name
	}{
		{
			def:       Default{Name: "deploy", Version: "1.0.0", File: "dongle-deploy", Requires: compat.Requires{Host: ">=0.1.0"}},
			payload:   "deploy-bin",
			wantEntry: "dongle-deploy", // no Entrypoint: the canonical dongle-<name>
		},
		{
			def:       Default{Name: "lint", Version: "0.2.0", File: "bin/lint", Entrypoint: "custom-lint"},
			payload:   "lint-bin",
			wantEntry: "custom-lint", // an explicit Entrypoint wins
		},
	}
	files := fstest.MapFS{}
	for _, c := range cases {
		files["embedded/"+c.def.File] = &fstest.MapFile{Data: []byte(c.payload)}
	}
	withPayload(t, files)

	for _, c := range cases {
		if err := InstallDefault(c.def); err != nil {
			t.Fatalf("InstallDefault(%s): %v", c.def.Name, err)
		}
	}

	st, err := state.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		p := filepath.Join(state.PluginVersionDir(c.def.Name, c.def.Version), c.wantEntry)
		b, err := os.ReadFile(p)
		if err != nil || string(b) != c.payload {
			t.Fatalf("%s: %q, %v", p, b, err)
		}
		if runtime.GOOS != "windows" { // Windows has no Unix permission bits
			if fi, _ := os.Stat(p); fi.Mode().Perm() != 0o755 {
				t.Fatalf("%s mode %v, want 0755", p, fi.Mode())
			}
		}
		want := state.Installed{Name: c.def.Name, ActiveVersion: c.def.Version, Entrypoint: c.wantEntry, Requires: c.def.Requires}
		if got := st.Plugins[c.def.Name]; got != want {
			t.Fatalf("state for %s = %+v, want %+v", c.def.Name, got, want)
		}
	}

	if err := InstallDefault(Default{Name: "ghost", Version: "1.0.0", File: "missing"}); err == nil {
		t.Fatal("want error for a default whose file wasn't staged")
	}
	if st, _ := state.Load(); len(st.Plugins) != len(cases) {
		t.Fatal("a failed default was recorded in state")
	}
}

func TestEmbeddedIndex(t *testing.T) {
	withPayload(t, fstest.MapFS{})
	if _, _, ok := EmbeddedIndex(); ok {
		t.Fatal("nothing staged: want ok=false")
	}
	withPayload(t, fstest.MapFS{"embedded/manifest.json": {Data: []byte(`{"plugins":[]}`)}})
	if _, _, ok := EmbeddedIndex(); ok {
		t.Fatal("no index version recorded: want ok=false")
	}
	withPayload(t, fstest.MapFS{"embedded/manifest.json": {Data: []byte(manifestJSON)}})
	if _, _, ok := EmbeddedIndex(); ok {
		t.Fatal("version recorded but archive missing: want ok=false")
	}
	withPayload(t, fstest.MapFS{
		"embedded/manifest.json": {Data: []byte(manifestJSON)},
		"embedded/index.tar.gz":  {Data: []byte("archive")},
	})
	archive, version, ok := EmbeddedIndex()
	if !ok || version != "2024.03.01" || string(archive) != "archive" {
		t.Fatalf("got %q, %q, %v", archive, version, ok)
	}
}
