package builtins

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/andreimladin/dongle/internal/index"
	"github.com/andreimladin/dongle/internal/ui"
)

// EnsureFresh is index.EnsureFresh(IndexTTL) for the builtins that only
// read the index (search, support): a spinner while the feed is contacted
// (shown only when it will be), and a failed refresh of a still-usable
// cache reported as a warning rather than an error.
func EnsureFresh() error {
	var sp *ui.Spinner
	if index.NeedsRefresh(IndexTTL) {
		sp = ui.StartSpinner("Refreshing plugin index...")
	}
	err := index.EnsureFresh(IndexTTL)
	if sp != nil {
		sp.Stop()
	}
	var stale *index.StaleError
	if errors.As(err, &stale) {
		ui.Warnf("%v", stale)
		return nil
	}
	return err
}

// Search shows what's available in the catalog (needs the index cache).
func Search() int {
	if err := EnsureFresh(); err != nil {
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
// plugin in the index (`dongle support`).
func Support(name string) int {
	if err := EnsureFresh(); err != nil {
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
