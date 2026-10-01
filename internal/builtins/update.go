package builtins

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

// SyncMode is what search/install/upgrade do when the feed has a newer index than
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

// Update brings the cached index up to date with the feed, ignoring the
// TTL (`dongle update`): it first asks the feed for the latest index
// version (metadata only), downloads the archive only when that version is
// newer than the cached one, and then prints the index version and the
// plugins it lists. On failure the existing cache stays in use — or, with
// nothing cached, the embedded seed is extracted — but the command still
// fails, since its one job didn't happen.
func Update() int {
	prev, _ := index.CachedVersion()
	sp := ui.StartSpinner("Checking for a newer plugin index...")
	v, err := index.LatestVersion()
	sp.Stop()
	if err != nil {
		return updateFailed(err)
	}

	if index.HasCache() && !index.IsNewer(v, prev) {
		index.MarkChecked()
		ui.Successf("Plugin index %s is already up to date.", prev)
		writeIndexSummary(os.Stdout, ui.Out, prev)
		return 0
	}

	sp = ui.StartSpinner(fmt.Sprintf("Downloading plugin index %s...", v))
	err = fetchAndApply(v)
	sp.Stop()
	if err != nil {
		return updateFailed(err)
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

// updateFailed reports Update's failure, falling back to the embedded
// seed when nothing is cached, and returns its exit code.
func updateFailed(err error) int {
	ui.Errorf("could not update the plugin index: %v", err)
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

// fetchAndApply downloads index version v from the feed and installs it
// as the cache.
func fetchAndApply(v string) error {
	latest, err := index.Fetch(v)
	if err != nil {
		return err
	}
	return latest.Apply()
}

// prepareIndex runs before search/install/upgrade act: it makes sure an index is
// cached, then asks the feed for its latest index version — a metadata
// query, no download — and compares it to the cached one. Only when the
// feed's is newer and, per mode (and whether the user can be prompted),
// the update is accepted is the index archive downloaded and swapped in;
// otherwise nothing is downloaded and the cached copy is used. It always
// tells the user (on stderr) which of those happened. Failing to reach the
// feed is not an error: the cached index is used and a warning says so.
// summary prints the updated index's plugin list after an update (search
// skips it, since listing the plugins is its own output).
func prepareIndex(mode SyncMode, summary bool) error {
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
	latest, err := index.LatestVersion()
	sp.Stop()
	if err != nil {
		ui.Warnf("could not check the feed for a newer plugin index: %v", err)
		ui.Infof("Using cached plugin index %s.", cached)
		return nil
	}

	if !index.IsNewer(latest, cached) {
		index.MarkChecked()
		ui.Successf("Plugin index %s is already up to date.", cached)
		return nil
	}

	update := false
	switch {
	case mode == SyncAlways:
		update = true
	case ui.StdinIsTerminal():
		update = ui.Confirm(fmt.Sprintf(
			"A newer plugin index is available (%s %s %s). Update the index first?", cached, ui.Err.Arrow(), latest))
	default:
		ui.Notef("a newer plugin index is available (%s -> %s); using the cached one.", cached, latest)
		ui.Notef("run `dongle update`, or pass --sync, to update it first.")
		return nil
	}
	if !update {
		ui.Infof("Using cached plugin index %s.", cached)
		return nil
	}

	sp = ui.StartSpinner(fmt.Sprintf("Downloading plugin index %s...", latest))
	err = fetchAndApply(latest)
	sp.Stop()
	if err != nil {
		ui.Warnf("could not update the plugin index: %v", err)
		ui.Infof("Using cached plugin index %s.", cached)
		return nil
	}
	cur, _ := index.CachedVersion()
	ui.Successf("Updated plugin index %s %s %s.", cached, ui.Err.Arrow(), cur)
	if summary {
		writeIndexSummary(os.Stderr, ui.Err, cached)
		fmt.Fprintln(os.Stderr)
	}
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
