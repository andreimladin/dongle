// Package hostcmd implements dongle's top-level builtins (`dongle
// version`, `dongle refresh`, `dongle support`), mirroring what
// internal/plugincmd does for `dongle plugin ...`: one exported function
// per command that validates its own args, does the work, prints its own
// output and errors, and returns the process exit code. cmd only wires
// each cobra command to its function.
package hostcmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/andreimladin/dongle/internal/index"
	"github.com/andreimladin/dongle/internal/plugincmd"
)

// Version handles `dongle version`: the host version and protocol, then
// the cached index's version and origin (or that none is cached yet).
// Extra args are ignored.
func Version(hostVersion, protocol string, args []string) int {
	fmt.Printf("dongle %s (protocol %s)\n", hostVersion, protocol)
	v, ok := index.CachedVersion()
	origin, _ := index.CachedOrigin()
	switch {
	case !ok:
		fmt.Println("index: not yet downloaded (run `dongle refresh` or any plugin command)")
	case origin == index.OriginEmbedded:
		fmt.Printf("index %s (embedded; run `dongle refresh` to check for updates)\n", v)
	default:
		fmt.Printf("index %s\n", v)
	}
	return 0
}

// Refresh handles `dongle refresh` (which replaces the old `dongle index
// refresh`): force-downloads the latest index archive from the feed,
// ignoring the TTL cache. Extra args are ignored.
func Refresh(args []string) int {
	if err := index.Refresh(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	v, _ := index.CachedVersion()
	if origin, ok := index.CachedOrigin(); ok && origin == index.OriginEmbedded {
		fmt.Printf("could not reach the feed; still on the embedded index %s\n", v)
	} else {
		fmt.Printf("index refreshed (%s)\n", v)
	}
	return 0
}

// Support handles `dongle support <plugin-name>`: reads a plugin's support
// links straight from the index manifest. It never consults installed
// state, so it works for any plugin in the index whether or not it's
// installed locally.
func Support(args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: dongle support <plugin-name>")
		return 1
	}
	name := args[0]

	if err := index.EnsureFresh(plugincmd.IndexTTL); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}

	m, err := index.Load(name)
	if errors.Is(err, index.ErrNotFound) {
		fmt.Fprintf(os.Stderr, "error: no plugin named %s in the index\n", name)
		return 1
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}

	fmt.Printf("%s — support\n", m.Name)
	fmt.Printf("  Documentation: %s\n", m.Support.Documentation)
	fmt.Printf("  Channel:       %s\n", m.Support.Channel)
	if m.Support.Contact != "" {
		fmt.Printf("  Contact:       %s\n", m.Support.Contact)
	}
	return 0
}
