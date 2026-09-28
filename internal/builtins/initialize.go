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
	seedIndex := !index.HasCache() && index.HasEmbedded()
	defaults := bootstrap.PendingDefaults()
	if !seedIndex && len(defaults) == 0 {
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
	if len(defaults) > 0 {
		if err := bootstrap.MarkDefaultsBootstrapped(); err != nil {
			ui.Warnf("could not record embedded defaults bootstrap: %v", err)
		}
	}
	ui.Successf("Initialization complete.")
}
