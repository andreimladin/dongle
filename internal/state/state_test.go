package state

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/andreimladin/dongle/internal/compat"
	"github.com/andreimladin/dongle/internal/host"
	"github.com/andreimladin/dongle/internal/testutil"
)

func TestDataDirIsUnderHome(t *testing.T) {
	testutil.Isolate(t)
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(home, "."+host.Name)
	if got := DataDir(); got != dir {
		t.Fatalf("DataDir() = %q, want %q", got, dir)
	}
	if got, want := PluginsDir(), filepath.Join(dir, "plugins"); got != want {
		t.Fatalf("PluginsDir() = %q, want %q", got, want)
	}
	if got, want := PluginVersionDir("deploy", "1.2.3"), filepath.Join(dir, "plugins", "deploy", "1.2.3"); got != want {
		t.Fatalf("PluginVersionDir = %q, want %q", got, want)
	}

	// The data dir used to be relocatable via DONGLE_DATA_DIR; it no
	// longer is.
	t.Setenv("DONGLE_DATA_DIR", t.TempDir())
	if got := DataDir(); got != dir {
		t.Fatalf("DONGLE_DATA_DIR must be ignored: DataDir() = %q, want %q", got, dir)
	}
}

func TestEntrypointName(t *testing.T) {
	if got, want := EntrypointName("deploy"), host.Name+"-deploy"; got != want {
		t.Fatalf("EntrypointName(deploy) = %q, want %q", got, want)
	}
}

func TestLoadMissingIsEmpty(t *testing.T) {
	testutil.Isolate(t)
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
	os.RemoveAll(testutil.Isolate(t))

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
	dir := testutil.Isolate(t)
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
		dir := testutil.Isolate(t)
		if err := os.WriteFile(filepath.Join(dir, "state.json"), []byte("{not json"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(); err == nil {
			t.Fatal("want error for corrupt state.json")
		}
	})
	t.Run("unreadable", func(t *testing.T) {
		dir := testutil.Isolate(t)
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
	dir := testutil.Isolate(t)
	os.RemoveAll(dir)
	if err := os.WriteFile(dir, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := (&State{}).Save(); err == nil {
		t.Fatal("want error saving under a regular file")
	}
}
