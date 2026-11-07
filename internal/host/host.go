// Package host is the single source of the host CLI's own name. Everything
// derived from it reads it from here: the command users type (in help and
// in hints like "run: dongle upgrade <plugin>"), the prefix of installed
// plugin entrypoints (dongle-<plugin>, see state.EntrypointName) and the
// data dir (~/.dongle, see state.DataDir).
//
// It is deliberately not used for the DONGLE_* environment variables the
// host sets for plugins: those are the host<->plugin protocol, which must
// not change when the host is renamed.
package host

// Name is the host CLI's name.
const Name = "dongle"
