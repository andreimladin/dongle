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

// This file is the terminal side of index.Prepare shared by every command
// that reads the index: spinners, the "update the index first?" prompt,
// and the index summary. The mechanics (TTL, feed check, download) live in
// internal/index; no command here depends on another command.

// SyncMode is what search/install/upgrade do when the feed has a newer index than
// the one cached (see prepareIndex).
type SyncMode int

const (
	// SyncAsk checks only once the cache is older than index.CheckTTL;
	// then install/upgrade prompt when stdin is a terminal, and otherwise
	// (and always, for search) keep the cached index and print a note
	// (never hangs a script).
	SyncAsk SyncMode = iota
	// SyncAlways checks regardless of the TTL and updates the index first
	// without asking (--sync).
	SyncAlways
	// SyncNever uses the cached index without checking the feed (--no-sync).
	SyncNever
)

// prepareIndex runs before search/install/upgrade act: index.Prepare
// makes sure an index is cached and, unless the cache is younger than
// index.CheckTTL (or mode is SyncNever), checks the feed for a newer
// index version — --sync (SyncAlways) checks regardless of the TTL.
// Within the TTL nothing is printed: the cache is used as-is.
//
// When the feed's index is newer, what happens depends on mutating:
// install/upgrade (true) download and swap it in once the update is
// accepted — per mode, and whether the user can be prompted — and then
// print the new index's plugin list; search (false) never prompts, just
// notes that a newer index exists and uses the cache (unless --sync).
// Otherwise nothing is downloaded and the cached copy is used. Failing to
// reach the feed is not an error: the cached index is used and a warning
// says so.
func prepareIndex(mode SyncMode, mutating bool) error {
	policy := index.CheckIfStale
	switch mode {
	case SyncAlways:
		policy = index.CheckAlways
	case SyncNever:
		policy = index.CheckNever
	}
	st, err := prepareWithProgress(policy)
	if err != nil {
		return err
	}
	cached := st.Cached
	switch {
	case st.Downloaded:
		ui.Successf("Downloaded plugin index %s.", cached)
		return nil
	case mode == SyncNever:
		ui.Infof("Using cached plugin index %s (--no-sync).", cached)
		return nil
	case st.CheckErr != nil:
		ui.Warnf("could not check the feed for a newer plugin index: %v", st.CheckErr)
		ui.Infof("Using cached plugin index %s.", cached)
		return nil
	case !st.Checked:
		// Within index.CheckTTL: the feed wasn't asked.
		return nil
	case !st.UpdateAvailable():
		ui.Successf("Plugin index %s is already up to date.", cached)
		return nil
	}
	latest := st.Latest

	update := false
	switch {
	case mode == SyncAlways:
		update = true
	case mutating && ui.StdinIsTerminal():
		update = ui.Confirm(fmt.Sprintf(
			"A newer plugin index is available (%s %s %s). Update the index first?", cached, ui.Err.Arrow(), latest))
	default: // search, or no terminal to prompt on
		ui.Notef("a newer plugin index is available (%s -> %s); using the cached one.", cached, latest)
		ui.Notef("run `dongle update`, or pass --sync, to update it first.")
		return nil
	}
	if !update {
		ui.Infof("Using cached plugin index %s.", cached)
		return nil
	}

	if err := downloadWithProgress(latest); err != nil {
		ui.Warnf("could not update the plugin index: %v", err)
		ui.Infof("Using cached plugin index %s.", cached)
		return nil
	}
	cur, _ := index.CachedVersion()
	ui.Successf("Updated plugin index %s %s %s.", cached, ui.Err.Arrow(), cur)
	if mutating {
		writeIndexSummary(os.Stderr, ui.Err, cached)
		fmt.Fprintln(os.Stderr)
	}
	return nil
}

// prepareWithProgress runs index.Prepare with a spinner for each slow
// step it reports.
func prepareWithProgress(policy index.Policy) (index.Status, error) {
	var sp *ui.Spinner
	st, err := index.Prepare(policy, func(s index.Step) {
		if sp != nil {
			sp.Stop()
		}
		switch s {
		case index.StepDownload:
			sp = ui.StartSpinner("Downloading plugin index...")
		case index.StepCheck:
			sp = ui.StartSpinner("Checking for a newer plugin index...")
		}
	})
	if sp != nil {
		sp.Stop()
	}
	return st, err
}

// downloadWithProgress runs index.Download(v) under a spinner.
func downloadWithProgress(v string) error {
	sp := ui.StartSpinner(fmt.Sprintf("Downloading plugin index %s...", v))
	err := index.Download(v)
	sp.Stop()
	return err
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
