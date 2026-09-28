package builtins

import (
	"github.com/andreimladin/dongle/internal/bootstrap"
	"github.com/andreimladin/dongle/internal/index"
	"github.com/andreimladin/dongle/internal/ui"
)

// Initialize is the first-run bootstrap of a batteries-included binary
// (built with -tags embed; see internal/bootstrap): it extracts the
// embedded plugin index into the cache and unpacks the embedded default
// plugins into the plugin store. It runs before every command but only
// does — and prints — anything on the first run, when there's something
// left to unpack; plain builds embed nothing, so for them it's a no-op.
//
// Progress goes to stderr (a spinner on a terminal, plain lines
// otherwise) so the user knows why the first run takes a moment, without
// touching the command's stdout.
func Initialize() {
	// Fast path for every run after the first: plain builds embed
	// nothing, and once the bootstrap is recorded and an index is cached
	// there's nothing left to unpack — decided from state alone, without
	// reading the embedded manifest.
	if !bootstrap.Embedded || (bootstrap.DefaultsBootstrapped() && index.HasCache()) {
		return
	}

	// Reading the embedded manifest (and the index archive it lists) is
	// itself slow in a large batteries-included binary, so it runs under
	// its own spinner rather than leaving the first run silent.
	sp := ui.StartSpinner("Initializing dongle...")
	seedIndex := !index.HasCache() && index.HasEmbedded()
	defaults := bootstrap.PendingDefaults()
	sp.Stop()
	if !seedIndex && len(defaults) == 0 {
		markDefaultsBootstrapped()
		return
	}

	if seedIndex {
		sp := ui.StartSpinner("Initializing plugin index...")
		if _, err := index.SeedEmbedded(); err != nil {
			sp.Stop()
			ui.Warnf("could not initialize the embedded plugin index: %v", err)
		} else {
			v, _ := index.CachedVersion()
			sp.Success("Initialized plugin index %s", v)
		}
	}
	for _, d := range defaults {
		sp := ui.StartSpinner("Installing " + d.Name + "...")
		if err := bootstrap.InstallDefault(d); err != nil {
			sp.Stop()
			ui.Warnf("installing embedded default %s: %v", d.Name, err)
			continue
		}
		sp.Success("Installed %s %s", d.Name, d.Version)
	}
	markDefaultsBootstrapped()
	ui.Successf("Initialization complete.")
}

// markDefaultsBootstrapped records the first-run bootstrap as done — even
// when this binary embeds no defaults — so later runs take Initialize's
// fast path.
func markDefaultsBootstrapped() {
	if bootstrap.DefaultsBootstrapped() {
		return
	}
	if err := bootstrap.MarkDefaultsBootstrapped(); err != nil {
		ui.Warnf("could not record embedded defaults bootstrap: %v", err)
	}
}
