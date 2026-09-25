// Package plugincmd implements dongle's plugin-management builtins —
// list, search, install, remove, upgrade, sync, support — plus the
// --version report and first-run initialization. Each exported function is the whole of one command: it prints
// its own results/errors and returns the process exit code, so cmd/ stays
// a thin cobra adapter.
package plugincmd

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/andreimladin/dongle/internal/compat"
	"github.com/andreimladin/dongle/internal/index"
	"github.com/andreimladin/dongle/internal/state"
	"github.com/andreimladin/dongle/internal/ui"
)

// IndexTTL is how long a cached index is trusted before commands that
// read it (search, support) refresh it from the feed. install and upgrade
// don't rely on it: they always check the feed for a newer index first
// (see prepareIndex). Exported so other builtins that read the index
// (e.g. `dongle support`) stay on the same freshness policy.
const IndexTTL = time.Hour

// Version prints the grouped `dongle --version` report: the host's own
// version, the index version in use, and every installed plugin.
func Version(hostVersion string) int {
	st, err := state.Load()
	if err != nil {
		ui.Errorf("reading state: %v", err)
		return 1
	}

	p := ui.Out
	var t ui.Table
	t.Style(0, p.Bold)
	t.Row(0, "dongle", hostVersion)
	iv, note := indexVersionLabel()
	t.Row(0, "index", iv, p.Dim(note))
	names := sortedNames(st)
	if len(names) == 0 {
		t.Heading(p.Bold("plugins:") + "  " + p.Dim("none installed"))
	} else {
		t.Heading(p.Bold("plugins:"))
		for _, n := range names {
			t.Row(2, n, st.Plugins[n].ActiveVersion)
		}
	}
	t.Write(os.Stdout)
	return 0
}

// indexVersionLabel describes the cached index for --version: its
// version, plus a note marking the embedded seed.
func indexVersionLabel() (version, note string) {
	v, ok := index.CachedVersion()
	if !ok {
		return "none", "(run `dongle sync`)"
	}
	if origin, _ := index.CachedOrigin(); origin == index.OriginEmbedded {
		return v, "(embedded)"
	}
	return v, ""
}

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

// List shows what's installed (from state.json — never touches the network).
func List() int {
	st, err := state.Load()
	if err != nil {
		ui.Errorf("%v", err)
		return 1
	}
	if len(st.Plugins) == 0 {
		ui.Infof("No plugins installed. Find some with `dongle search`.")
		return 0
	}
	var t ui.Table
	t.Style(0, ui.Out.Bold)
	for _, n := range sortedNames(st) {
		t.Row(0, n, st.Plugins[n].ActiveVersion)
	}
	t.Write(os.Stdout)
	return 0
}

// EnsureFresh is index.EnsureFresh(IndexTTL) for the builtins that only
// read the index (search, support): a spinner while the feed is contacted
// (shown only when it will be), and a failed refresh of a still-usable
// cache reported as a warning rather than an error.
func EnsureFresh() error {
	var sp *ui.Spinner
	if index.NeedsRefresh(IndexTTL) {
		sp = ui.StartSpinner("Refreshing plugin index...")
	}
	err := index.EnsureFresh(IndexTTL)
	if sp != nil {
		sp.Stop()
	}
	var stale *index.StaleError
	if errors.As(err, &stale) {
		ui.Warnf("%v", stale)
		return nil
	}
	return err
}

// Search shows what's available in the catalog (needs the index cache).
func Search() int {
	if err := EnsureFresh(); err != nil {
		ui.Errorf("%v", err)
		return 1
	}
	entries, err := index.List()
	if err != nil {
		ui.Errorf("%v", err)
		return 1
	}
	if len(entries) == 0 {
		ui.Infof("The plugin index is empty.")
		return 0
	}
	sortManifests(entries)
	var t ui.Table
	t.Style(0, ui.Out.Bold)
	t.Style(2, ui.Out.Dim)
	for _, e := range entries {
		t.Row(0, e.Name, strings.TrimPrefix(e.Version, "v"), e.ShortDescription)
	}
	t.Write(os.Stdout)
	return 0
}

// Support prints where to get help with a plugin, straight from its index
// manifest — installed state is never consulted, so it works for any
// plugin in the index (`dongle support`).
func Support(name string) int {
	if err := EnsureFresh(); err != nil {
		ui.Errorf("%v", err)
		return 1
	}
	m, err := index.Load(name)
	if errors.Is(err, index.ErrNotFound) {
		ui.Errorf("no plugin named %s in the index (see `dongle search`)", name)
		return 1
	}
	if err != nil {
		ui.Errorf("%v", err)
		return 1
	}

	fmt.Println(ui.Out.Bold(m.Name) + " — support")
	var t ui.Table
	t.Style(0, ui.Out.Dim)
	t.Row(2, "Documentation:", m.Support.Documentation)
	t.Row(2, "Channel:", m.Support.Channel)
	if m.Support.Contact != "" {
		t.Row(2, "Contact:", m.Support.Contact)
	}
	t.Write(os.Stdout)
	return 0
}

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

// Install resolves name from the index and installs it. mode decides what
// happens when the feed has a newer index than the cache (see
// prepareIndex).
func Install(hostVersion, protocol, name string, mode SyncMode) int {
	if err := prepareIndex(mode); err != nil {
		ui.Errorf("%v", err)
		return 1
	}

	m, err := index.Load(name)
	if errors.Is(err, index.ErrNotFound) {
		ui.Errorf("no plugin named %s in the index (see `dongle search`)", name)
		return 1
	}
	if err != nil {
		ui.Errorf("%v", err)
		return 1
	}
	return installResolved(hostVersion, protocol, m, "")
}

// installResolved compat-checks, downloads and places the plugin m
// describes. A non-empty fromVersion makes it an upgrade from that
// version, which only changes the progress and success messages.
func installResolved(hostVersion, protocol string, m *index.Manifest, fromVersion string) int {
	if ok, reason, err := compat.Check(hostVersion, protocol, m.Requires); err != nil {
		ui.Errorf("bad constraint in manifest: %v", err)
		return 1
	} else if !ok {
		ui.Errorf("%s %s — not installing", m.Name, reason)
		return 1
	}

	plat, err := m.PlatformFor()
	if err != nil {
		ui.Errorf("%v", err)
		return 1
	}

	version := strings.TrimPrefix(m.Version, "v")
	action := fmt.Sprintf("Installing %s %s...", m.Name, version)
	if fromVersion != "" {
		action = fmt.Sprintf("Upgrading %s %s %s %s...", m.Name, fromVersion, ui.Err.Arrow(), version)
	}
	sp := ui.StartSpinner(action)
	staging, downloaded, err := fetchArtifact(m, plat)
	if staging != "" {
		defer os.RemoveAll(staging)
	}
	if err == nil {
		err = placePlugin(m, downloaded)
	}
	sp.Stop()
	if err != nil {
		ui.Errorf("%v", err)
		return 1
	}

	if fromVersion != "" {
		ui.Resultf("Upgraded %s %s %s %s", ui.Out.Bold(m.Name), fromVersion, ui.Out.Arrow(), version)
	} else {
		ui.Resultf("Installed %s %s %s", ui.Out.Bold(m.Name), version,
			ui.Out.Dim(fmt.Sprintf("(run it with: %s %s)", hostBinaryName(), m.Name)))
	}
	return 0
}

// stagingRoot returns a fresh temp dir under <dataDir>/.staging so that the
// final os.Rename into plugins/<name>/<version>/ is same-filesystem.
func stagingRoot() (string, error) {
	base := filepath.Join(state.DataDir(), ".staging")
	if err := os.MkdirAll(base, 0o755); err != nil {
		return "", err
	}
	return os.MkdirTemp(base, "install-")
}

// fetchArtifact downloads the artifact from the Azure feed into
// staging/download, ready for placePlugin. There is no archive to unpack:
// the downloaded item is the plugin binary itself. staging is always
// returned (even on error) so the caller can clean it up.
func fetchArtifact(m *index.Manifest, plat *index.Platform) (staging, downloaded string, err error) {
	staging, err = stagingRoot()
	if err != nil {
		return "", "", err
	}
	dlDir := filepath.Join(staging, "download")

	downloaded, err = downloadArtifact(m, plat, dlDir)
	if err != nil {
		return staging, "", err
	}
	return staging, downloaded, nil
}

// downloadArtifact shells out to the Azure CLI to pull the platform's
// Universal Package into destDir. plat.Package is the Universal Package
// name (already matched to this machine's os/arch by PlatformFor()) — not a
// filename. The package contains exactly one file, whose name is not
// predictable (it need not equal plat.Package), so downloadArtifact reads
// destDir afterward and returns the path to whichever single regular file
// landed there.
func downloadArtifact(m *index.Manifest, plat *index.Platform, destDir string) (string, error) {
	if _, err := exec.LookPath("az"); err != nil {
		return "", fmt.Errorf("the Azure CLI is required: install it and run `az extension add --name azure-devops`")
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", err
	}
	ver := strings.TrimPrefix(m.Version, "v") // upack versions are bare semver
	args := []string{
		"artifacts", "universal", "download",
		"--organization", "https://dev.azure.com/" + m.Feed.Organization,
		"--feed", m.Feed.Feed,
		"--name", plat.Package,
		"--version", ver,
		"--path", destDir,
	}
	if m.Feed.Project != "" {
		args = append(args, "--project", m.Feed.Project, "--scope", "project")
	}
	// az's stderr is captured (not inherited) so it can't garble the
	// spinner; index.AzError surfaces it if the download fails.
	var stderr bytes.Buffer
	cmd := exec.Command("az", args...)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", index.AzError(fmt.Sprintf("az download %s@%s", plat.Package, ver), err, stderr.Bytes())
	}

	entries, err := os.ReadDir(destDir)
	if err != nil {
		return "", err
	}
	var files []string
	for _, e := range entries {
		if e.Type().IsRegular() {
			files = append(files, e.Name())
		}
	}
	if len(files) != 1 {
		return "", fmt.Errorf("package %s@%s contains %d files, expected exactly 1", plat.Package, ver, len(files))
	}
	return filepath.Join(destDir, files[0]), nil
}

// placePlugin is not a user entry point: it only receives an
// already-resolved downloaded file produced by fetchArtifact, and the index
// resolver (Install) is its only caller. There is no plugin.json to
// read — name, version, and requires all come from the manifest. The
// downloaded file's own name is not predictable (it need not match anything
// in the manifest), so placePlugin canonicalizes it to a stable entrypoint
// name — <host binary name>-<plugin name> — before placing it, so dispatch
// always knows what to exec regardless of how the package itself named the
// file. downloaded already lives under <dataDir>/.staging (from
// fetchArtifact), so the final move into the plugins dir is a same-filesystem
// rename and state is only updated once the plugin is fully on disk. It
// prints nothing (it runs under the install spinner); errors are returned.
func placePlugin(m *index.Manifest, downloaded string) error {
	if err := os.Chmod(downloaded, 0o755); err != nil {
		return err
	}

	version := strings.TrimPrefix(m.Version, "v")
	entrypoint := hostBinaryName() + "-" + m.Name
	verDir := state.PluginVersionDir(m.Name, version)
	dst := filepath.Join(verDir, entrypoint)

	if err := os.RemoveAll(verDir); err != nil {
		return err
	}
	if err := os.MkdirAll(verDir, 0o755); err != nil {
		return err
	}
	if err := os.Rename(downloaded, dst); err != nil {
		// cross-filesystem safety net: staging is normally under the data dir
		// (same filesystem as dst), but fall back to a copy if it isn't.
		if cerr := copyFile(downloaded, dst, 0o755); cerr != nil {
			return fmt.Errorf("place plugin: %w", cerr)
		}
	}

	st, err := state.Load()
	if err != nil {
		return err
	}
	st.Plugins[m.Name] = state.Installed{
		Name:          m.Name,
		ActiveVersion: version,
		Entrypoint:    entrypoint,
		Requires:      m.Requires,
	}
	return st.Save()
}

// hostBinaryName returns the currently running host binary's own basename,
// so a renamed host binary (built as something other than "dongle") gets
// plugin entrypoints named to match: <hostBinaryName>-<plugin>. Falls back
// to "dongle" if the binary name can't be determined.
func hostBinaryName() string {
	name := filepath.Base(os.Args[0])
	if name == "" || name == "." || name == string(filepath.Separator) {
		return "dongle"
	}
	return name
}

// Remove deletes an installed plugin (every version on disk) and drops it
// from state.
func Remove(name string) int {
	st, err := state.Load()
	if err != nil {
		ui.Errorf("%v", err)
		return 1
	}
	if _, ok := st.Plugins[name]; !ok {
		ui.Errorf("%s is not installed", name)
		return 1
	}
	if err := os.RemoveAll(filepath.Join(state.PluginsDir(), name)); err != nil {
		ui.Errorf("%v", err)
		return 1
	}
	delete(st.Plugins, name)
	if err := st.Save(); err != nil {
		ui.Errorf("%v", err)
		return 1
	}
	ui.Resultf("Removed %s", ui.Out.Bold(name))
	return 0
}

// --- helpers ------------------------------------------------------------------

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Chmod(mode)
}
