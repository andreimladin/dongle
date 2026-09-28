//go:build !embed

package bootstrap

// Plain builds embed nothing, so there are no defaults to unpack and no
// seed index. See bootstrap.go (compiled only with -tags embed) for the
// real implementations.

// PendingDefaults reports no embedded defaults in plain builds.
func PendingDefaults() []Default { return nil }

// InstallDefault is unreachable in plain builds (PendingDefaults is empty).
func InstallDefault(Default) error { return nil }

// MarkDefaultsBootstrapped is a no-op in plain builds.
func MarkDefaultsBootstrapped() error { return nil }

// EmbeddedIndex reports no seed index in plain builds.
func EmbeddedIndex() (archive []byte, version string, ok bool) { return nil, "", false }
