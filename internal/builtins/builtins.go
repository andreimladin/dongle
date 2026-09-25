// Package builtins implements dongle's builtin commands — search,
// install, remove, upgrade, sync, support — plus the --version report and
// first-run initialization. Each exported function is the whole of one
// command: it prints its own results/errors and returns the process exit
// code, so cmd/ stays a thin cobra adapter. The mechanisms underneath
// (index fetch/cache, installed state, embedded defaults) live in
// internal/index, internal/state and internal/bootstrap; this package is
// the user-facing layer on top of them.
package builtins

import (
	"sort"
	"time"

	"github.com/andreimladin/dongle/internal/index"
	"github.com/andreimladin/dongle/internal/state"
)

// IndexTTL is how long a cached index is trusted before commands that
// read it (search, support) refresh it from the feed. install and upgrade
// don't rely on it: they always check the feed for a newer index first
// (see prepareIndex). Exported so other builtins that read the index
// (e.g. `dongle support`) stay on the same freshness policy.
const IndexTTL = time.Hour

func sortManifests(ms []index.Manifest) {
	sort.Slice(ms, func(i, j int) bool { return ms[i].Name < ms[j].Name })
}

func sortedNames(st *state.State) []string {
	names := make([]string, 0, len(st.Plugins))
	for n := range st.Plugins {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
