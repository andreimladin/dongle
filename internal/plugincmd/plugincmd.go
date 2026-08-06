// Package plugincmd implements the `dongle plugin ...` builtins.
package plugincmd

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
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

	entry, err := index.Manifest(name)
	if errors.Is(err, index.ErrNotFound) {
		// A miss is exactly when a stale cache is the likely cause — force a
		// refresh and try once more before giving up.
		if rerr := index.Refresh(); rerr == nil {
			entry, err = index.Manifest(name)
		}
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	if ok, reason, err := compat.Check(hostVersion, protocol, entry.Requires); err != nil {
		fmt.Fprintln(os.Stderr, "error: bad constraint in index manifest:", err)
		return 1
	} else if !ok {
		fmt.Fprintf(os.Stderr, "error: %s %s — not installing\n", entry.Name, reason)
		return 1
	}

	plat, err := entry.PlatformFor()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}

	staging, unpacked, err := fetchAndUnpack(entry, plat)
	if staging != "" {
		defer os.RemoveAll(staging)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}

	return placePlugin(entry, plat, unpacked)
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

// fetchAndUnpack downloads the artifact from the Azure feed, verifies its
// checksum, and unpacks it into staging/unpacked, ready for placePlugin.
// staging is always returned (even on error) so the caller can clean it up.
func fetchAndUnpack(e *index.PluginIndexEntry, plat *index.Platform) (staging, unpacked string, err error) {
	staging, err = stagingRoot()
	if err != nil {
		return "", "", err
	}
	dl := filepath.Join(staging, "download")
	unpacked = filepath.Join(staging, "unpacked")

	tarPath, err := downloadArtifact(e, plat, dl)
	if err != nil {
		return staging, "", err
	}
	if err := verifySHA256(tarPath, plat.SHA256); err != nil {
		return staging, "", err
	}
	if err := untar(tarPath, unpacked); err != nil {
		return staging, "", err
	}
	return staging, unpacked, nil
}

// downloadArtifact shells out to the Azure CLI to pull the plugin's Universal
// Package into destDir, then returns the path to plat.File within it.
func downloadArtifact(e *index.PluginIndexEntry, plat *index.Platform, destDir string) (string, error) {
	if _, err := exec.LookPath("az"); err != nil {
		return "", fmt.Errorf("the Azure CLI is required: install it and run `az extension add --name azure-devops`")
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", err
	}
	ver := strings.TrimPrefix(e.Version, "v") // upack versions are bare semver
	args := []string{
		"artifacts", "universal", "download",
		"--organization", "https://dev.azure.com/" + e.Feed.Organization,
		"--feed", e.Feed.Feed,
		"--name", e.Feed.PackageName,
		"--version", ver,
		"--path", destDir,
	}
	if e.Feed.Project != "" {
		args = append(args, "--project", e.Feed.Project, "--scope", "project")
	}
	cmd := exec.Command("az", args...)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("az download %s@%s: %w", e.Feed.PackageName, ver, err)
	}
	tarPath := filepath.Join(destDir, plat.File)
	if _, err := os.Stat(tarPath); err != nil {
		return "", fmt.Errorf("expected %s in package %s@%s, not found", plat.File, e.Feed.PackageName, ver)
	}
	return tarPath, nil
}

// placePlugin is not a user entry point: it only receives an
// already-resolved/unpacked dir produced by fetchAndUnpack, and the index
// resolver (installFromName) is its only caller. There is no plugin.json to
// read — name, version, entrypoint, and requires all come from the index
// entry and the platform selected for this download. It places the unpacked
// payload atomically (same-filesystem rename, since unpacked lives under the
// staging root) and only updates state once the plugin is fully on disk.
func placePlugin(entry *index.PluginIndexEntry, plat *index.Platform, unpacked string) int {
	entrypoint := filepath.Join(unpacked, plat.Bin)
	if _, err := os.Stat(entrypoint); err != nil {
		fmt.Fprintf(os.Stderr, "error: entrypoint %q not found in package for %s %s\n", plat.Bin, entry.Name, entry.Version)
		return 1
	}
	if err := os.Chmod(entrypoint, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}

	version := strings.TrimPrefix(entry.Version, "v")
	dst := state.PluginVersionDir(entry.Name, version)
	if err := os.RemoveAll(dst); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	if err := os.Rename(unpacked, dst); err != nil {
		// cross-filesystem safety net: staging is normally under the data dir
		// (same filesystem as dst), but fall back to a copy if it isn't.
		if cerr := copyTree(unpacked, dst); cerr != nil {
			fmt.Fprintln(os.Stderr, "error: place plugin:", cerr)
			return 1
		}
	}

	st, err := state.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	st.Plugins[entry.Name] = state.Installed{
		Name:          entry.Name,
		ActiveVersion: version,
		Entrypoint:    plat.Bin,
		Requires:      entry.Requires,
	}
	if err := st.Save(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}

	fmt.Printf("installed %s %s (command: dongle %s)\n", entry.Name, version, entry.Name)
	return 0
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

// copyTree recursively copies src into dst, preserving file modes. It's the
// EXDEV fallback for placePlugin when staging and the plugins dir aren't on
// the same filesystem and os.Rename can't be used.
func copyTree(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		return copyFile(path, target, info.Mode())
	})
}

func verifySHA256(path, want string) error {
	if want == "" {
		return fmt.Errorf("no sha256 in manifest for %s", filepath.Base(path))
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(got, want) {
		return fmt.Errorf("checksum mismatch: got %s, want %s", got, want)
	}
	return nil
}

// untar extracts a .tar.gz into dest, guarding against path traversal.
func untar(tarGzPath, dest string) error {
	f, err := os.Open(tarGzPath)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		target := filepath.Join(dest, hdr.Name)
		// zip-slip guard: ensure target stays within dest.
		if !strings.HasPrefix(target, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("unsafe path in archive: %s", hdr.Name)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
		}
	}
}
