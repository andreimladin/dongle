// Package plugincmd implements the `dongle plugin ...` builtins.
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
)

const indexTTL = 24 * time.Hour

// Run handles `dongle plugin <subcommand>`.
func Run(hostVersion, protocol string, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: dongle plugin <list|search|install|uninstall> ...")
		return 2
	}
	switch args[0] {
	case "list":
		return list()
	case "search":
		return search()
	case "install":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: dongle plugin install <name>")
			return 2
		}
		return install(hostVersion, protocol, args[1])
	case "uninstall":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: dongle plugin uninstall <name>")
			return 2
		}
		return uninstall(args[1])
	default:
		fmt.Fprintf(os.Stderr, "unknown plugin subcommand %q\n", args[0])
		return 2
	}
}

// list shows what's installed (from state.json — never touches the network).
func list() int {
	st, err := state.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	if len(st.Plugins) == 0 {
		fmt.Println("no plugins installed")
		return 0
	}
	names := make([]string, 0, len(st.Plugins))
	for n := range st.Plugins {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		fmt.Printf("%-16s %s\n", n, st.Plugins[n].ActiveVersion)
	}
	return 0
}

// search shows what's available in the catalog (needs the index cache).
func search() int {
	if err := index.EnsureFresh(indexTTL); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	entries, err := index.List()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	for _, e := range entries {
		fmt.Printf("%-16s %-10s %s\n", e.Name, e.Version, e.ShortDescription)
	}
	return 0
}

// install resolves name from the index and installs it.
func install(hostVersion, protocol, name string) int {
	return installFromName(hostVersion, protocol, name)
}

func installFromName(hostVersion, protocol, name string) int {
	if err := index.EnsureFresh(indexTTL); err != nil {
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
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

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

	return placePlugin(m, downloaded)
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
// resolver (installFromName) is its only caller. There is no plugin.json to
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

	fmt.Printf("installed %s %s (command: dongle %s)\n", m.Name, version, m.Name)
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

func uninstall(name string) int {
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
	fmt.Printf("uninstalled %s\n", name)
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
