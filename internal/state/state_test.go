package state

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/andreimladin/dongle/internal/compat"
)

func TestDataDirOverrideAndDefault(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DONGLE_DATA_DIR", dir)
	if got := DataDir(); got != dir {
		t.Fatalf("DataDir() = %q, want %q", got, dir)
	}
	if got, want := PluginVersionDir("deploy", "1.2.3"), filepath.Join(dir, "plugins", "deploy", "1.2.3"); got != want {
		t.Fatalf("PluginVersionDir = %q, want %q", got, want)
	}

	home := t.TempDir()
	t.Setenv("DONGLE_DATA_DIR", "")
	t.Setenv("HOME", home)
	if got, want := DataDir(), filepath.Join(home, ".dongle"); got != want {
		t.Fatalf("default DataDir() = %q, want %q", got, want)
	}
}

func TestLoadMissingIsEmpty(t *testing.T) {
	t.Setenv("DONGLE_DATA_DIR", t.TempDir())
	st, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if st.Plugins == nil || len(st.Plugins) != 0 || st.DefaultsBootstrapped {
		t.Fatalf("got %+v, want an empty, writable state", st)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	// Save must create a data dir that doesn't exist yet.
	dir := filepath.Join(t.TempDir(), "nested", "data")
	t.Setenv("DONGLE_DATA_DIR", dir)

	want := &State{
		Plugins: map[string]Installed{
			"deploy": {Name: "deploy", ActiveVersion: "1.2.3", Entrypoint: "dongle-deploy",
				Requires: compat.Requires{Host: ">=0.1.0", Protocol: "v1"}},
		},
		DefaultsBootstrapped: true,
	}
	if err := want.Save(); err != nil {
		t.Fatal(err)
	}
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip: got %+v, want %+v", got, want)
	}
}

func TestLoadNullPluginsIsWritable(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DONGLE_DATA_DIR", dir)
	if err := os.WriteFile(filepath.Join(dir, "state.json"), []byte(`{"plugins":null}`), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	st.Plugins["x"] = Installed{Name: "x"} // would panic on a nil map
}

func TestLoadErrors(t *testing.T) {
	t.Run("corrupt json", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("DONGLE_DATA_DIR", dir)
		if err := os.WriteFile(filepath.Join(dir, "state.json"), []byte("{not json"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(); err == nil {
			t.Fatal("want error for corrupt state.json")
		}
	})
	t.Run("unreadable", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("DONGLE_DATA_DIR", dir)
		// A directory where the file should be: exists, but can't be read.
		if err := os.Mkdir(filepath.Join(dir, "state.json"), 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(); err == nil {
			t.Fatal("want error for unreadable state.json")
		}
	})
}

func TestSaveFailsWhenDataDirIsAFile(t *testing.T) {
	f := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(f, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DONGLE_DATA_DIR", f)
	if err := (&State{}).Save(); err == nil {
		t.Fatal("want error saving under a regular file")
	}
}
