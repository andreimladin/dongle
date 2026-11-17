package builtins

import (
	"fmt"
	"os"

	"github.com/andreimladin/dongle/internal/index"
	"github.com/andreimladin/dongle/internal/state"
	"github.com/andreimladin/dongle/internal/ui"
)

// Version prints the `dongle --version` report: the host's own version
// and the index version in use as aligned key/value lines, then a blank
// line and every installed plugin (or "no plugins installed").
func Version(hostVersion string) int {
	st, err := state.Load()
	if err != nil {
		ui.Errorf("reading state: %v", err)
		return 1
	}

	var top ui.Table
	top.Row(0, "dongle", hostVersion)
	iv := "none"
	if v, ok := index.CachedVersion(); ok {
		iv = v
	}
	top.Row(0, "index", iv)
	top.Write(os.Stdout)

	fmt.Println()
	names := sortedNames(st)
	if len(names) == 0 {
		fmt.Println("no plugins installed")
		return 0
	}
	fmt.Println("installed plugins:")
	var plugins ui.Table
	for _, n := range names {
		plugins.Row(2, n, st.Plugins[n].ActiveVersion)
	}
	plugins.Write(os.Stdout)
	return 0
}
