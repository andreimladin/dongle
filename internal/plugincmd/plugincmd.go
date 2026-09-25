// Package plugincmd implements dongle's plugin-management builtins —
// list, search, install, remove, upgrade, sync — plus the --version
// report. Each exported function is the whole of one command: it prints
// its own results/errors and returns the process exit code, so cmd/ stays
// a thin cobra adapter.
package plugincmd

import (
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

// IndexTTL is how long a cloned index cache is trusted before commands that
// read it force a refresh. Exported so other builtins that read the index
// (e.g. `dongle support`) stay on the same freshness policy.
const IndexTTL = 24 * time.Hour

// Version prints the grouped `dongle --version` report: the host's own
// version, the index version in use, and every installed plugin.
func Version(hostVersion, protocol string) int {
	st, err := state.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error: reading state:", err)
		return 1
	}

	var t ui.Table
	t.Row(0, "dongle", hostVersion+"  (protocol "+protocol+")")
	t.Row(0, "index", indexVersionLabel())
	names := sortedNames(st)
	if len(names) == 0 {
		t.Heading("plugins:  none installed")
	} else {
		t.Heading("plugins:")
		for _, n := range names {
			t.Row(2, n, st.Plugins[n].ActiveVersion)
		}
	}
	t.Write(os.Stdout)
	return 0
}

// indexVersionLabel describes the cached index for --version.
func indexVersionLabel() string {
	v, ok := index.CachedVersion()
	if !ok {
		return "not downloaded yet (run `dongle sync`)"
	}
	if origin, _ := index.CachedOrigin(); origin == index.OriginEmbedded {
		return v + "  (embedded)"
	}
	return v
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
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	if len(st.Plugins) == 0 {
		fmt.Fprintln(os.Stderr, "No plugins installed. Find some with `dongle search`.")
		return 0
	}
	var t ui.Table
	for _, n := range sortedNames(st) {
		t.Row(0, n, st.Plugins[n].ActiveVersion)
	}
	t.Write(os.Stdout)
	return 0
}

// Search shows what's available in the catalog (needs the index cache).
func Search() int {
	if err := index.EnsureFresh(IndexTTL); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	entries, err := index.List()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	if len(entries) == 0 {
		fmt.Fprintln(os.Stderr, "The plugin index is empty.")
		return 0
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	var t ui.Table
	for _, e := range entries {
		t.Row(0, e.Name, e.Version, e.ShortDescription)
	}
	t.Write(os.Stdout)
	return 0
}

// Sync force-downloads the latest index from the feed (`dongle sync`).
func Sync() int {
	if err := index.Refresh(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	v, _ := index.CachedVersion()
	if origin, ok := index.CachedOrigin(); ok && origin == index.OriginEmbedded {
		fmt.Printf("could not reach the feed; still on the embedded index %s\n", v)
	} else {
		fmt.Printf("index synced (%s)\n", v)
	}
	return 0
}

// Upgrade brings installed plugins up to the versions the index currently
// declares: just name when it's non-empty, otherwise every installed
// plugin. Only installed plugins are considered, and a plugin whose
// installed version is ahead of the index is never downgraded.
func Upgrade(hostVersion, protocol, name string) int {
	st, err := state.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	if name != "" {
		if _, ok := st.Plugins[name]; !ok {
			fmt.Fprintf(os.Stderr, "error: %s is not installed; use `dongle install %s`\n", name, name)
			return 1
		}
	} else if len(st.Plugins) == 0 {
		fmt.Fprintln(os.Stderr, "No plugins installed; nothing to upgrade.")
		return 0
	}

	if err := index.EnsureFresh(IndexTTL); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
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
	fmt.Fprintf(os.Stderr, "\n%d upgraded, %d already up to date, %d skipped, %d failed\n",
		upgraded, current, skipped, failed)
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
			fmt.Fprintf(os.Stderr, "skipped %s: "+format+"\n", append([]any{name}, a...)...)
			return 0, upgradeSkipped
		}
		fmt.Fprintf(os.Stderr, "error: "+format+"\n", a...)
		return 1, upgradeSkipped
	}

	m, err := index.Load(name)
	if errors.Is(err, index.ErrNotFound) {
		return skipOrFail("%s is not in the index", name)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s: %v\n", name, err)
		return 1, upgradeSkipped
	}

	cmp, err := compat.CompareVersions(m.Version, inst.ActiveVersion)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s: %v\n", name, err)
		return 1, upgradeSkipped
	}
	switch {
	case cmp == 0:
		fmt.Printf("%s is already up to date (%s)\n", name, inst.ActiveVersion)
		return 0, upgradeCurrent
	case cmp < 0:
		return skipOrFail("installed %s %s is newer than the index (%s); not downgrading. Use remove + install to force.",
			name, inst.ActiveVersion, m.Version)
	}

	if code := installResolved(hostVersion, protocol, m, "upgraded", inst.ActiveVersion); code != 0 {
		return code, upgradeSkipped
	}
	return 0, upgradeDone
}

// Install resolves name from the index and installs it.
func Install(hostVersion, protocol, name string) int {
	if err := index.EnsureFresh(IndexTTL); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}

	m, err := index.Load(name)
	if errors.Is(err, index.ErrNotFound) {
		// A miss is exactly when a stale cache is the likely cause — force a
		// refresh and try once more before giving up.
		if rerr := index.Refresh(); rerr == nil {
			m, err = index.Load(name)
		}
	}
	if errors.Is(err, index.ErrNotFound) {
		fmt.Fprintf(os.Stderr, "error: no plugin named %s in the index (see `dongle search`)\n", name)
		return 1
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	return installResolved(hostVersion, protocol, m, "installed", "")
}

// installResolved compat-checks, downloads and places the plugin m
// describes. verb and fromVersion only shape the success message
// ("installed x 1.2.0" vs "upgraded x 1.1.0 -> 1.2.0").
func installResolved(hostVersion, protocol string, m *index.Manifest, verb, fromVersion string) int {
	if ok, reason, err := compat.Check(hostVersion, protocol, m.Requires); err != nil {
		fmt.Fprintln(os.Stderr, "error: bad constraint in manifest:", err)
		return 1
	} else if !ok {
		fmt.Fprintf(os.Stderr, "error: %s %s — not installing\n", m.Name, reason)
		return 1
	}

	plat, err := m.PlatformFor()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}

	staging, downloaded, err := fetchArtifact(m, plat)
	if staging != "" {
		defer os.RemoveAll(staging)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}

	if code := placePlugin(m, downloaded); code != 0 {
		return code
	}
	version := strings.TrimPrefix(m.Version, "v")
	if fromVersion != "" {
		fmt.Printf("%s %s %s -> %s\n", verb, m.Name, fromVersion, version)
	} else {
		fmt.Printf("%s %s %s (run it with: %s %s)\n", verb, m.Name, version, hostBinaryName(), m.Name)
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
	cmd := exec.Command("az", args...)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("az download %s@%s: %w", plat.Package, ver, err)
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
// rename and state is only updated once the plugin is fully on disk.
func placePlugin(m *index.Manifest, downloaded string) int {
	if err := os.Chmod(downloaded, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}

	version := strings.TrimPrefix(m.Version, "v")
	entrypoint := hostBinaryName() + "-" + m.Name
	verDir := state.PluginVersionDir(m.Name, version)
	dst := filepath.Join(verDir, entrypoint)

	if err := os.RemoveAll(verDir); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	if err := os.MkdirAll(verDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	if err := os.Rename(downloaded, dst); err != nil {
		// cross-filesystem safety net: staging is normally under the data dir
		// (same filesystem as dst), but fall back to a copy if it isn't.
		if cerr := copyFile(downloaded, dst, 0o755); cerr != nil {
			fmt.Fprintln(os.Stderr, "error: place plugin:", cerr)
			return 1
		}
	}

	st, err := state.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	st.Plugins[m.Name] = state.Installed{
		Name:          m.Name,
		ActiveVersion: version,
		Entrypoint:    entrypoint,
		Requires:      m.Requires,
	}
	if err := st.Save(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	return 0
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
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	if _, ok := st.Plugins[name]; !ok {
		fmt.Fprintf(os.Stderr, "error: %s is not installed\n", name)
		return 1
	}
	if err := os.RemoveAll(filepath.Join(state.PluginsDir(), name)); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	delete(st.Plugins, name)
	if err := st.Save(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	fmt.Printf("removed %s\n", name)
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
