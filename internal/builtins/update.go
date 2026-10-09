package builtins

import (
	"os"

	"github.com/andreimladin/dongle/internal/index"
	"github.com/andreimladin/dongle/internal/ui"
)

// Update brings the cached index up to date with the feed, ignoring
// index.CheckTTL (`dongle update`): index.Prepare(CheckAlways) asks the
// feed for the latest index version (metadata only), the archive is
// downloaded only when that version is newer than the cached one, and
// then the index version and the plugins it lists are printed. On failure
// the existing cache stays in use — or, with nothing cached, the embedded
// seed is extracted — but the command still fails, since its one job
// didn't happen.
func Update() int {
	prev, _ := index.CachedVersion()
	st, err := prepareWithProgress(index.CheckAlways)
	if err == nil {
		err = st.CheckErr
	}
	if err != nil {
		return updateFailed(err)
	}

	if !st.Downloaded && !st.UpdateAvailable() {
		ui.Successf("Plugin index %s is already up to date.", prev)
		writeIndexSummary(os.Stdout, ui.Out, prev)
		return 0
	}

	if !st.Downloaded {
		if err := downloadWithProgress(st.Latest); err != nil {
			return updateFailed(err)
		}
	}
	cur, _ := index.CachedVersion()
	switch {
	case prev == "" || prev == cur:
		ui.Successf("Plugin index %s is the latest.", cur)
	default:
		ui.Successf("Updated plugin index %s %s %s.", prev, ui.Err.Arrow(), cur)
	}
	writeIndexSummary(os.Stdout, ui.Out, prev)
	return 0
}

// updateFailed reports Update's failure, falling back to the embedded
// seed when nothing is cached, and returns its exit code.
func updateFailed(err error) int {
	ui.Errorf("could not update the plugin index: %v", err)
	if !index.HasCache() {
		if _, serr := index.SeedEmbedded(); serr != nil {
			ui.Warnf("could not seed the embedded plugin index: %v", serr)
		}
	}
	if v, ok := index.CachedVersion(); ok {
		ui.Infof("Still using plugin index %s.", v)
	}
	return 1
}
