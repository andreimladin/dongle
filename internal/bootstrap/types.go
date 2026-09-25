package bootstrap

import "github.com/andreimladin/dongle/internal/compat"

// Default is one default plugin embedded in a batteries-included binary:
// an entry of embedded/manifest.json's "plugins" list, written by
// scripts/build.sh's fetch_embedded from configs/build.yaml. Defined
// outside the build-tagged files so plain builds share the type.
type Default struct {
	Name       string          `json:"name"`
	Version    string          `json:"version"`
	File       string          `json:"file"`
	Entrypoint string          `json:"entrypoint,omitempty"`
	Requires   compat.Requires `json:"requires,omitempty"`
}
