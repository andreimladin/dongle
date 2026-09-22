//go:build !embed

package bootstrap

// InstallDefaults is a no-op in plain builds: nothing is embedded, so
// there's nothing to unpack on first run. See bootstrap.go (compiled only
// with -tags embed) for the real implementation.
func InstallDefaults() {}

// EmbeddedIndex is a no-op in plain builds: nothing is embedded, so there
// is no seed index for internal/index to extract. See bootstrap.go
// (compiled only with -tags embed) for the real implementation.
func EmbeddedIndex() (archive []byte, version string, ok bool) { return nil, "", false }
