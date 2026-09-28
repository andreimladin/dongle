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
// read the index (search, support), with progress shown while any slow
// step runs: seeding the cache from the embedded index on a first run, or
// refreshing it from the feed once it's older than the TTL. A failed
// refresh of a still-usable cache is reported as a warning rather than an
// error.
func EnsureFresh() error {
	if !index.HasCache() {
		return initIndex()
	}
	if !index.NeedsRefresh(IndexTTL) {
		return nil
	}
	sp := ui.StartSpinner("Refreshing plugin index...")
	err := index.EnsureFresh(IndexTTL)
	var stale *index.StaleError
	switch {
	case errors.As(err, &stale):
		sp.Warn("%v", stale)
		return nil
	case err != nil:
		sp.Stop()
		return err
	}
	v, _ := index.CachedVersion()
	sp.Success("Refreshed plugin index %s", v)
	return nil
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
