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
	sp := ui.StartSpinner("Syncing plugin index...")
	latest, err := index.FetchLatest()
	if err == nil {
		err = latest.Apply()
	}
	sp.Stop()
	if err != nil {
		ui.Errorf("could not sync the plugin index: %v", err)
		if !index.HasCache() {
			if _, serr := index.SeedEmbedded(); serr != nil {
				ui.Warnf("could not seed the embedded plugin index: %v", serr)
			}
		}
		if v, ok := index.CachedVersion(); ok {
			ui.Infof("Still using plugin index %s.", v)
		}
		return 1
	}
	cur, _ := index.CachedVersion()
	switch {
	case prev == "" || prev == cur:
		ui.Successf("Plugin index %s is the latest.", cur)
	default:
		ui.Successf("Updated plugin index %s %s %s.", prev, ui.Err.Arrow(), cur)
	}
	writeIndexSummary(os.Stdout, ui.Out, prev)
	return 0
}

// prepareIndex runs before install/upgrade act: it makes sure an index is
// cached, then checks the feed for a newer one and — per mode, and
// whether the user can be prompted — either updates the cache first or
// keeps using the cached copy. It always tells the user (on stderr) which
// of those happened. Failing to reach the feed is not an error: the
// cached index is used and a warning says so.
func prepareIndex(mode SyncMode) error {
	if !index.HasCache() && !index.HasEmbedded() {
		// Nothing cached and nothing embedded to seed from: the first
		// download is the latest by definition, so there's nothing newer
		// to check for afterwards.
		sp := ui.StartSpinner("Downloading plugin index...")
		err := index.EnsureCache()
		sp.Stop()
		if err != nil {
			return err
		}
		v, _ := index.CachedVersion()
		ui.Successf("Downloaded plugin index %s.", v)
		return nil
	}
	if err := index.EnsureCache(); err != nil {
		return err
	}
	cached, _ := index.CachedVersion()
	if mode == SyncNever {
		ui.Infof("Using cached plugin index %s (--no-sync).", cached)
		return nil
	}

	sp := ui.StartSpinner("Checking for a newer plugin index...")
	latest, err := index.FetchLatest()
	sp.Stop()
	if err != nil {
		ui.Warnf("could not check the feed for a newer plugin index: %v", err)
		ui.Infof("Using cached plugin index %s.", cached)
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
		ui.Successf("Plugin index %s is already the latest.", cached)
		return nil
	}

	update := false
	switch {
	case mode == SyncAlways:
		update = true
	case ui.StdinIsTerminal():
		update = ui.Confirm(fmt.Sprintf(
			"A newer plugin index is available (%s %s %s). Update the index first?", cached, ui.Err.Arrow(), latest.Version))
	default:
		latest.Discard()
		ui.Notef("a newer plugin index is available (%s -> %s); using the cached one.", cached, latest.Version)
		ui.Notef("run `dongle sync`, or pass --sync, to update it first.")
		return nil
	}
	if !update {
		latest.Discard()
		ui.Infof("Using cached plugin index %s.", cached)
		return nil
	}

	if err := latest.Apply(); err != nil {
		ui.Warnf("could not update the plugin index: %v", err)
		ui.Infof("Using cached plugin index %s.", cached)
		return nil
	}
	ui.Successf("Updated plugin index %s %s %s.", cached, ui.Err.Arrow(), latest.Version)
	writeIndexSummary(os.Stderr, ui.Err, cached)
	fmt.Fprintln(os.Stderr)
	return nil
}

// writeIndexSummary prints the cached index's version (noting what it was
// updated from, when prev differs) and every plugin it lists, marking the
// installed ones and those with an upgrade available.
// p is the palette for w's stream.
func writeIndexSummary(w io.Writer, p ui.Palette, prev string) {
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
	t.Style(0, p.Bold)
	// The note is always the last cell (never padded), so it can be styled
	// per row without skewing alignment.
	t.Row(0, "index", cur, p.Dim(note))
	entries, err := index.List()
	if err != nil || len(entries) == 0 {
		t.Heading(p.Bold("plugins:") + "  none")
		t.Write(w)
		return
	}
	st, err := state.Load()
	if err != nil {
		st = &state.State{}
	}
	sortManifests(entries)
	t.Heading(p.Bold("plugins:"))
	for _, e := range entries {
		t.Row(2, e.Name, strings.TrimPrefix(e.Version, "v"), installedNote(p, st, e))
	}
	t.Write(w)
}

// installedNote describes a listed plugin's local install state.
func installedNote(p ui.Palette, st *state.State, m index.Manifest) string {
	inst, ok := st.Plugins[m.Name]
	if !ok {
		return ""
	}
	if cmp, err := compat.CompareVersions(m.Version, inst.ActiveVersion); err == nil && cmp > 0 {
		return p.Yellow("installed " + inst.ActiveVersion + ", upgrade available")
	}
	return p.Green("installed")
}
