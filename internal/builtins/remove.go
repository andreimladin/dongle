package builtins

import (
	"os"
	"path/filepath"

	"github.com/andreimladin/dongle/internal/state"
	"github.com/andreimladin/dongle/internal/ui"
)

// Remove deletes an installed plugin (every version on disk) and drops it
// from state.
func Remove(name string) int {
	st, err := state.Load()
	if err != nil {
		ui.Errorf("%v", err)
		return 1
	}
	if _, ok := st.Plugins[name]; !ok {
		ui.Errorf("%s is not installed", name)
		return 1
	}
	if err := os.RemoveAll(filepath.Join(state.PluginsDir(), name)); err != nil {
		ui.Errorf("%v", err)
		return 1
	}
	delete(st.Plugins, name)
	if err := st.Save(); err != nil {
		ui.Errorf("%v", err)
		return 1
	}
	ui.Resultf("Removed %s", ui.Out.Bold(name))
	return 0
}
