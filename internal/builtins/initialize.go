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
	seedIndex := !index.HasCache() && payload.HasIndex()
	sp := ui.StartSpinner("Initializing dongle...")
	defaults := payload.PendingDefaults()
	sp.Stop()
	if !seedIndex && len(defaults) == 0 {
		return
	}

	if seedIndex {
		sp := ui.StartSpinner("Initializing plugin index...")
		if err := payload.SeedIndex(); err != nil {
			sp.Stop()
			ui.Warnf("could not initialize the embedded plugin index: %v", err)
		} else {
			v, _ := index.CachedVersion()
			sp.Success("Initialized plugin index %s", v)
		}
	}
	for _, d := range defaults {
		sp := ui.StartSpinner("Installing " + d.Name + "...")
		if err := payload.InstallDefault(d); err != nil {
			sp.Stop()
			ui.Warnf("installing embedded default %s: %v", d.Name, err)
			continue
		}
		sp.Success("Installed %s %s", d.Name, d.Version)
	}
	if len(defaults) > 0 {
		if err := payload.MarkDefaultsBootstrapped(); err != nil {
			ui.Warnf("could not record embedded defaults bootstrap: %v", err)
		}
	}
	ui.Successf("Initialization complete.")
}

// embeddedPayload is what Initialize unpacks on a batteries-included
// binary's first run: the seed index (via internal/index) and the default
// plugins (via internal/bootstrap). Tests swap in a fake.
type embeddedPayload interface {
	HasIndex() bool
	SeedIndex() error
	PendingDefaults() []bootstrap.Default
	InstallDefault(bootstrap.Default) error
	MarkDefaultsBootstrapped() error
}

var payload embeddedPayload = builtPayload{}

// builtPayload is the payload actually compiled into this binary.
type builtPayload struct{}

func (builtPayload) HasIndex() bool { return index.HasEmbedded() }

func (builtPayload) SeedIndex() error {
	_, err := index.SeedEmbedded()
	return err
}

func (builtPayload) PendingDefaults() []bootstrap.Default { return bootstrap.PendingDefaults() }

func (builtPayload) InstallDefault(d bootstrap.Default) error { return bootstrap.InstallDefault(d) }

func (builtPayload) MarkDefaultsBootstrapped() error { return bootstrap.MarkDefaultsBootstrapped() }
