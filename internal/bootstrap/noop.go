//go:build !embed

package bootstrap

// InstallDefaults is a no-op in plain builds: nothing is embedded, so
// there's nothing to unpack on first run. See bootstrap.go (compiled only
// with -tags embed) for the real implementation.
func InstallDefaults() {}
