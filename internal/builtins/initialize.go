package builtins

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/andreimladin/dongle/internal/index"
	"github.com/andreimladin/dongle/internal/state"
	"github.com/andreimladin/dongle/internal/ui"
)

// Initialize is dongle's first-run bootstrap. The binary carries no
// plugins and no index, so on first run (state's DefaultsBootstrapped
// flag unset) it:
//
//  1. downloads the latest plugin index from the feed (index.Fetch),
//  2. reads the index's top-level defaults file (index.DefaultsFile) and
//     installs each plugin it lists through the normal install path
//     (download from the feed, place, register in state),
//  3. swaps the downloaded index in as the cache and sets
//     DefaultsBootstrapped, so this happens exactly once.
//
// It runs before every command (help included) but does — and prints — nothing once
// initialized. Progress goes to stderr (a spinner on a terminal, plain
// lines otherwise), never stdout.
//
// First run requires feed access. If the feed can't be reached (or a
// default plugin can't be downloaded), Initialize reports it, removes the
// default plugins it already placed, leaves the cache and flag untouched
// and returns a nonzero exit code, so the next run starts over cleanly.
// A default that can never be installed here — not in the index,
// incompatible with this host, or no build for this os/arch — is skipped
// with a warning instead: retrying wouldn't help.
//
// A build with no index feed identity (a plain `go build ./cmd` without
// DONGLE_INDEX_* set) has nothing to download from, so for it Initialize
// is a no-op.
func Initialize(hostVersion, protocol string) int {
	st, err := state.Load()
	if err != nil {
		ui.Errorf("reading state: %v", err)
		return 1
	}
	if st.DefaultsBootstrapped || !index.Configured() {
		return 0
	}

	sp := ui.StartSpinner("Downloading plugin index...")
	latest, err := index.Fetch("*")
	if err != nil {
		sp.Stop()
		return initFailed(err)
	}
	defer latest.Discard() // no-op once applied
	sp.Success("Downloaded plugin index %s", latest.Version)

	names, err := latest.Defaults()
	if err != nil {
		ui.Errorf("reading the plugin index's default plugins: %v", err)
		ui.Infof("Nothing was changed; run dongle again to retry.")
		return 1
	}

	var added []state.Installed // placed by this run, removed again on failure
	for _, name := range names {
		if _, ok := st.Plugins[name]; ok {
			continue // already installed (e.g. by hand before first run)
		}
		m, err := latest.Load(name)
		if err != nil {
			ui.Warnf("skipping default plugin %s: %v", name, err)
			continue
		}
		plat, err := checkInstallable(hostVersion, protocol, m)
		if err != nil {
			ui.Warnf("skipping default plugin %s: %v", name, err)
			continue
		}

		version := strings.TrimPrefix(m.Version, "v")
		sp := ui.StartSpinner("Installing " + name + "...")
		staging, downloaded, err := fetchArtifact(m, plat)
		if err != nil {
			sp.Stop()
			os.RemoveAll(staging)
			rollbackDefaults(added)
			return initFailed(err)
		}
		err = placePlugin(m, downloaded)
		os.RemoveAll(staging)
		if err != nil {
			sp.Stop()
			rollbackDefaults(append(added, state.Installed{Name: name, ActiveVersion: version}))
			ui.Errorf("installing default plugin %s: %v", name, err)
			ui.Infof("Nothing was changed; run dongle again to retry.")
			return 1
		}
		added = append(added, state.Installed{Name: name, ActiveVersion: version})
		sp.Success("Installed %s %s", name, version)
	}

	if err := latest.Apply(); err != nil {
		rollbackDefaults(added)
		ui.Errorf("saving the plugin index: %v", err)
		ui.Infof("Nothing was changed; run dongle again to retry.")
		return 1
	}
	if st, err = state.Load(); err == nil {
		st.DefaultsBootstrapped = true
		err = st.Save()
	}
	if err != nil {
		// Plugins and index are in place; only the flag is missing, so the
		// next run simply redoes the (idempotent) bootstrap.
		ui.Errorf("recording initialization: %v", err)
		return 1
	}
	ui.Successf("Initialization complete.")
	return 0
}

// initFailed reports a first-run failure to reach the feed and returns
// the exit code. Nothing has been changed by then (see rollbackDefaults).
func initFailed(err error) int {
	ui.Errorf("couldn't reach the feed to initialize dongle — check your connection and that you're signed in (az login)")
	fmt.Fprintf(os.Stderr, "  cause: %s\n", strings.ReplaceAll(err.Error(), "\n", "\n  "))
	ui.Infof("Nothing was changed; run dongle again to retry.")
	return 1
}

// rollbackDefaults removes the default plugins a failed first run already
// placed — their files and their state entries — so no half-initialized
// state is left behind. Best effort: failures are ignored, since the
// bootstrap flag stays unset and the next run reinstalls anyway.
func rollbackDefaults(added []state.Installed) {
	if len(added) == 0 {
		return
	}
	for _, p := range added {
		os.RemoveAll(state.PluginVersionDir(p.Name, p.ActiveVersion))
		os.Remove(filepath.Join(state.PluginsDir(), p.Name)) // only if now empty
	}
	st, err := state.Load()
	if err != nil {
		return
	}
	for _, p := range added {
		delete(st.Plugins, p.Name)
	}
	_ = st.Save()
}
