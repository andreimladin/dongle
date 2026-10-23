//go:build embed

package bootstrap

import (
	"os"
	"path/filepath"
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
	t.Setenv("DONGLE_DATA_DIR", t.TempDir())

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
	t.Setenv("DONGLE_DATA_DIR", t.TempDir())
	orig := os.Args[0]
	os.Args[0] = "/usr/bin/acme"
	defer func() { os.Args[0] = orig }()
	withPayload(t, fstest.MapFS{
		"embedded/dongle-deploy": {Data: []byte("deploy-bin")},
		"embedded/bin/lint":      {Data: []byte("lint-bin")},
	})

	// Default entrypoint follows the host binary's name.
	if err := InstallDefault(Default{Name: "deploy", Version: "1.0.0", File: "dongle-deploy",
		Requires: compat.Requires{Host: ">=0.1.0"}}); err != nil {
		t.Fatal(err)
	}
	if err := InstallDefault(Default{Name: "lint", Version: "0.2.0", File: "bin/lint", Entrypoint: "custom-lint"}); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ name, ver, entry, body string }{
		{"deploy", "1.0.0", "acme-deploy", "deploy-bin"},
		{"lint", "0.2.0", "custom-lint", "lint-bin"},
	} {
		p := filepath.Join(state.PluginVersionDir(c.name, c.ver), c.entry)
		b, err := os.ReadFile(p)
		if err != nil || string(b) != c.body {
			t.Fatalf("%s: %q, %v", p, b, err)
		}
		if fi, _ := os.Stat(p); fi.Mode().Perm() != 0o755 {
			t.Fatalf("%s mode %v", p, fi.Mode())
		}
	}
	st, _ := state.Load()
	if st.Plugins["deploy"].Entrypoint != "acme-deploy" || st.Plugins["deploy"].Requires.Host != ">=0.1.0" ||
		st.Plugins["lint"].ActiveVersion != "0.2.0" {
		t.Fatalf("state = %+v", st.Plugins)
	}

	if err := InstallDefault(Default{Name: "ghost", Version: "1.0.0", File: "missing"}); err == nil {
		t.Fatal("want error for a default whose file wasn't staged")
	}
	if st, _ := state.Load(); len(st.Plugins) != 2 {
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
