package builtins

import (
	"os"

	"github.com/andreimladin/dongle/internal/index"
	"github.com/andreimladin/dongle/internal/state"
	"github.com/andreimladin/dongle/internal/ui"
)

// Version prints the grouped `dongle --version` report: the host's own
// version, the index version in use, and every installed plugin.
func Version(hostVersion string) int {
	st, err := state.Load()
	if err != nil {
		ui.Errorf("reading state: %v", err)
		return 1
	}

	p := ui.Out
	var t ui.Table
	t.Style(0, p.Bold)
	t.Row(0, "dongle", hostVersion)
	iv, note := indexVersionLabel()
	t.Row(0, "index", iv, p.Dim(note))
	names := sortedNames(st)
	if len(names) == 0 {
		t.Heading(p.Bold("plugins:") + "  " + p.Dim("none installed"))
	} else {
		t.Heading(p.Bold("plugins:"))
		for _, n := range names {
			t.Row(2, n, st.Plugins[n].ActiveVersion)
		}
	}
	t.Write(os.Stdout)
	return 0
}

// indexVersionLabel describes the cached index for --version: its
// version, plus a note marking the embedded seed.
func indexVersionLabel() (version, note string) {
	v, ok := index.CachedVersion()
	if !ok {
		return "none", "(run `dongle sync`)"
	}
	if origin, _ := index.CachedOrigin(); origin == index.OriginEmbedded {
		return v, "(embedded)"
	}
	return v, ""
}
