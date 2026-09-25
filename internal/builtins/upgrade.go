package builtins

import (
	"errors"
	"fmt"
	"os"

	"github.com/andreimladin/dongle/internal/compat"
	"github.com/andreimladin/dongle/internal/index"
	"github.com/andreimladin/dongle/internal/state"
	"github.com/andreimladin/dongle/internal/ui"
)

// Upgrade brings installed plugins up to the versions the index currently
// declares: just name when it's non-empty, otherwise every installed
// plugin. Only installed plugins are considered, and a plugin whose
// installed version is ahead of the index is never downgraded.
// mode decides what happens when the feed has a newer index than the
// cache (see prepareIndex).
func Upgrade(hostVersion, protocol, name string, mode SyncMode) int {
	st, err := state.Load()
	if err != nil {
		ui.Errorf("%v", err)
		return 1
	}
	if name != "" {
		if _, ok := st.Plugins[name]; !ok {
			ui.Errorf("%s is not installed; use `dongle install %s`", name, name)
			return 1
		}
	} else if len(st.Plugins) == 0 {
		ui.Infof("No plugins installed; nothing to upgrade.")
		return 0
	}

	if err := prepareIndex(mode); err != nil {
		ui.Errorf("%v", err)
		return 1
	}

	if name != "" {
		code, _ := upgradeOne(hostVersion, protocol, st.Plugins[name], false)
		return code
	}

	var upgraded, current, skipped, failed int
	for _, n := range sortedNames(st) {
		code, res := upgradeOne(hostVersion, protocol, st.Plugins[n], true)
		switch {
		case code != 0:
			failed++
		case res == upgradeDone:
			upgraded++
		case res == upgradeCurrent:
			current++
		default:
			skipped++
		}
	}
	fmt.Fprintln(os.Stderr)
	summary := fmt.Sprintf("%d upgraded, %d already up to date, %d skipped, %d failed",
		upgraded, current, skipped, failed)
	if failed > 0 {
		ui.Errorf("%s", summary)
	} else {
		ui.Successf("%s", summary)
	}
	if failed > 0 {
		return 1
	}
	return 0
}

type upgradeResult int

const (
	upgradeDone upgradeResult = iota
	upgradeCurrent
	upgradeSkipped
)

// upgradeOne upgrades a single installed plugin to the index's version.
// With all set (part of `dongle upgrade` with no name), conditions that
// are errors for an explicit single-plugin upgrade — not in the index,
// installed version ahead of the index — are reported as skips instead,
// so one odd plugin doesn't fail the whole run.
func upgradeOne(hostVersion, protocol string, inst state.Installed, all bool) (int, upgradeResult) {
	name := inst.Name
	skipOrFail := func(format string, a ...any) (int, upgradeResult) {
		if all {
			ui.Warnf("skipping "+format, a...)
			return 0, upgradeSkipped
		}
		ui.Errorf(format, a...)
		return 1, upgradeSkipped
	}

	m, err := index.Load(name)
	if errors.Is(err, index.ErrNotFound) {
		return skipOrFail("%s is not in the index", name)
	}
	if err != nil {
		ui.Errorf("%s: %v", name, err)
		return 1, upgradeSkipped
	}

	cmp, err := compat.CompareVersions(m.Version, inst.ActiveVersion)
	if err != nil {
		ui.Errorf("%s: %v", name, err)
		return 1, upgradeSkipped
	}
	switch {
	case cmp == 0:
		fmt.Println(ui.Out.Dim(fmt.Sprintf("%s is already up to date (%s)", name, inst.ActiveVersion)))
		return 0, upgradeCurrent
	case cmp < 0:
		return skipOrFail("installed %s %s is newer than the index (%s); not downgrading. Use remove + install to force.",
			name, inst.ActiveVersion, m.Version)
	}

	if code := installResolved(hostVersion, protocol, m, inst.ActiveVersion); code != 0 {
		return code, upgradeSkipped
	}
	return 0, upgradeDone
}
