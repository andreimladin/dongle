package plugincmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/andreimladin/dongle/internal/compat"
	"github.com/andreimladin/dongle/internal/index"
	"github.com/andreimladin/dongle/internal/state"
	"github.com/andreimladin/dongle/internal/ui"
)

// SyncMode is what install/upgrade do when the feed has a newer index than
// the one cached (see prepareIndex).
type SyncMode int

const (
	// SyncAsk prompts when stdin is a terminal, and otherwise keeps the
	// cached index and prints a note (never hangs a script).
	SyncAsk SyncMode = iota
	// SyncAlways updates the index first without asking (--sync).
	SyncAlways
	// SyncNever uses the cached index without checking the feed (--no-sync).
	SyncNever
)

// Sync downloads the latest index from the feed into the cache, ignoring
// the TTL, and prints the index version and the plugins it lists
// (`dongle sync`). On failure the existing cache stays in use — or, with
// nothing cached, the embedded seed is extracted — but the command still
// fails, since its one job didn't happen.
func Sync() int {
	prev, _ := index.CachedVersion()
	latest, err := index.FetchLatest()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error: could not download the latest plugin index:", err)
		if !index.HasCache() {
			index.SeedEmbedded()
		}
		if v, ok := index.CachedVersion(); ok {
			fmt.Fprintf(os.Stderr, "Still using plugin index %s.\n", v)
		}
		return 1
	}
	if err := latest.Apply(); err != nil {
		fmt.Fprintln(os.Stderr, "error: installing the downloaded plugin index:", err)
		return 1
	}
	writeIndexSummary(os.Stdout, prev)
	return 0
}

// prepareIndex runs before install/upgrade act: it makes sure an index is
// cached, then checks the feed for a newer one and — per mode, and
// whether the user can be prompted — either updates the cache first or
// keeps using the cached copy. It always tells the user (on stderr) which
// of those happened. Failing to reach the feed is not an error: the
// cached index is used and a warning says so.
func prepareIndex(mode SyncMode) error {
	hadCache := index.HasCache()
	if err := index.EnsureCache(); err != nil {
		return err
	}
	cached, _ := index.CachedVersion()
	if origin, _ := index.CachedOrigin(); !hadCache && origin == index.OriginFetched {
		// No cache and nothing embedded: EnsureCache just downloaded the
		// latest from the feed, so there is nothing newer to check for.
		fmt.Fprintf(os.Stderr, "Downloaded plugin index %s.\n", cached)
		return nil
	}
	if mode == SyncNever {
		fmt.Fprintf(os.Stderr, "Using cached plugin index %s (--no-sync).\n", cached)
		return nil
	}

	latest, err := index.FetchLatest()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not check the feed for a newer plugin index: %v\n", err)
		fmt.Fprintf(os.Stderr, "Using cached plugin index %s.\n", cached)
		return nil
	}

	if !index.IsNewer(latest.Version, cached) {
		if latest.Version == cached {
			// Same content; applying it just records that the cache is now
			// feed-confirmed (no longer "embedded") and restarts its TTL.
			if err := latest.Apply(); err != nil {
				index.MarkChecked()
			}
		} else {
			latest.Discard()
			index.MarkChecked()
		}
		fmt.Fprintf(os.Stderr, "Plugin index %s is already the latest.\n", cached)
		return nil
	}

	update := false
	switch {
	case mode == SyncAlways:
		update = true
	case ui.StdinIsTerminal():
		update = ui.Confirm(fmt.Sprintf(
			"A newer plugin index is available (%s -> %s). Update the index first?", cached, latest.Version))
	default:
		latest.Discard()
		fmt.Fprintf(os.Stderr,
			"note: a newer plugin index is available (%s -> %s); using the cached one.\n"+
				"      Run `dongle sync`, or pass --sync, to update it first.\n", cached, latest.Version)
		return nil
	}
	if !update {
		latest.Discard()
		fmt.Fprintf(os.Stderr, "Using cached plugin index %s.\n", cached)
		return nil
	}

	if err := latest.Apply(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not update the plugin index: %v\n", err)
		fmt.Fprintf(os.Stderr, "Using cached plugin index %s.\n", cached)
		return nil
	}
	writeIndexSummary(os.Stderr, cached)
	fmt.Fprintln(os.Stderr)
	return nil
}

// writeIndexSummary prints the cached index's version (noting what it was
// updated from, when prev differs) and every plugin it lists, marking the
// installed ones and those with an upgrade available.
func writeIndexSummary(w io.Writer, prev string) {
	cur, _ := index.CachedVersion()
	note := ""
	switch {
	case prev == "":
	case prev == cur:
		note = "(already the latest)"
	default:
		note = "(updated from " + prev + ")"
	}

	var t ui.Table
	t.Row(0, "index", cur, note)
	entries, err := index.List()
	if err != nil || len(entries) == 0 {
		t.Heading("plugins:  none")
		t.Write(w)
		return
	}
	st, err := state.Load()
	if err != nil {
		st = &state.State{}
	}
	sortManifests(entries)
	t.Heading("plugins:")
	for _, e := range entries {
		t.Row(2, e.Name, strings.TrimPrefix(e.Version, "v"), installedNote(st, e))
	}
	t.Write(w)
}

// installedNote describes a listed plugin's local install state.
func installedNote(st *state.State, m index.Manifest) string {
	inst, ok := st.Plugins[m.Name]
	if !ok {
		return ""
	}
	if cmp, err := compat.CompareVersions(m.Version, inst.ActiveVersion); err == nil && cmp > 0 {
		return "installed " + inst.ActiveVersion + ", upgrade available"
	}
	return "installed"
}
