//go:build !embed

package main

// installEmbeddedDefaults is a no-op in plain builds: nothing is embedded,
// so there's nothing to unpack on first run. See cmd/embed.go (compiled
// only with -tags embed) for the real implementation.
func installEmbeddedDefaults() {}
