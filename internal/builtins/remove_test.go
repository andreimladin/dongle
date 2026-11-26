package builtins

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/andreimladin/dongle/internal/state"
	"github.com/andreimladin/dongle/internal/testutil"
)

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
