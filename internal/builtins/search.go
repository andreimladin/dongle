package builtins

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/andreimladin/dongle/internal/index"
	"github.com/andreimladin/dongle/internal/ui"
)

// Search shows what's available in the catalog. Like install/upgrade it
// first checks the feed for a newer index version once the cache is older
// than index.CheckTTL, but it never prompts: a newer index is only noted
// (or downloaded with --sync) — see prepareIndex.
func Search(mode SyncMode) int {
	if err := prepareIndex(mode, false); err != nil {
		ui.Errorf("%v", err)
		return 1
	}
	entries, err := index.List()
	if err != nil {
		ui.Errorf("%v", err)
		return 1
	}
	if len(entries) == 0 {
		ui.Infof("The plugin index is empty.")
		return 0
	}
	sortManifests(entries)
	var t ui.Table
	t.Style(0, ui.Out.Bold)
	t.Style(2, ui.Out.Dim)
	for _, e := range entries {
		t.Row(0, e.Name, strings.TrimPrefix(e.Version, "v"), e.ShortDescription)
	}
	t.Write(os.Stdout)
	return 0
}

// Support prints where to get help with a plugin, straight from its index
// manifest — installed state is never consulted, so it works for any
// plugin in the index (`dongle support`). It reads the cached index as-is
// and never contacts the feed (support links rarely change; `dongle
// update` or a search/install refreshes the index).
func Support(name string) int {
	if err := index.EnsureCache(); err != nil {
		ui.Errorf("%v", err)
		return 1
	}
	m, err := index.Load(name)
	if errors.Is(err, index.ErrNotFound) {
		ui.Errorf("no plugin named %s in the index (see `dongle search`)", name)
		return 1
	}
	if err != nil {
		ui.Errorf("%v", err)
		return 1
	}

	fmt.Println(ui.Out.Bold(m.Name) + " — support")
	var t ui.Table
	t.Style(0, ui.Out.Dim)
	t.Row(2, "Documentation:", m.Support.Documentation)
	t.Row(2, "Channel:", m.Support.Channel)
	if m.Support.Contact != "" {
		t.Row(2, "Contact:", m.Support.Contact)
	}
	t.Write(os.Stdout)
	return 0
}
