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
	iv, note := indexVersionLabel()
	top.Row(0, "index", iv+suffix(note))
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

// suffix returns note preceded by a space, or "" when note is empty.
func suffix(note string) string {
	if note == "" {
		return ""
	}
	return " " + note
}

// indexVersionLabel describes the cached index for --version: its
// version, plus a note marking the embedded seed.
func indexVersionLabel() (version, note string) {
	v, ok := index.CachedVersion()
	if !ok {
		return "none", "(run `dongle update`)"
	}
	if origin, _ := index.CachedOrigin(); origin == index.OriginEmbedded {
		return v, "(embedded)"
	}
	return v, ""
}
