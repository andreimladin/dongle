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
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/andreimladin/dongle/internal/compat"
	"github.com/andreimladin/dongle/internal/index"
	"github.com/andreimladin/dongle/internal/manifest"
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
			fmt.Fprintln(os.Stderr, "usage: dongle plugin install <name|dir>")
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

// install accepts either a local build dir (contains plugin.json + binary) or a
// bare plugin name to resolve from the index.
func install(hostVersion, protocol, arg string) int {
	if fi, err := os.Stat(arg); err == nil && fi.IsDir() {
		return installFromDir(hostVersion, protocol, arg)
	}
	return installFromName(hostVersion, protocol, arg)
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

	tmpDir, err := fetchAndUnpack(entry, plat)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	defer os.RemoveAll(tmpDir)

	return installFromDir(hostVersion, protocol, tmpDir)
}

// fetchAndUnpack downloads the artifact from the Azure feed, verifies its
// checksum, and unpacks it into a temp dir ready for installFromDir.
//
// TODO(feed): implement downloadArtifact using the Azure CLI for the first cut:
//
//	az artifacts universal download \
//	  --organization https://dev.azure.com/<org> \
//	  --feed <feed> --name <packageName> --version <version> \
//	  --path <tmp>
//
// then select plat.File from the downloaded package. Later, swap for the Azure
// DevOps Artifacts REST API with a brokered Entra token to drop the az
// dependency. verifySHA256 and untar below are already the real implementations.
func fetchAndUnpack(e *index.PluginIndexEntry, plat *index.Platform) (string, error) {
	tarPath, err := downloadArtifact(e, plat)
	if err != nil {
		return "", err
	}
	if err := verifySHA256(tarPath, plat.SHA256); err != nil {
		return "", err
	}
	dest, err := os.MkdirTemp("", "dongle-unpack-")
	if err != nil {
		return "", err
	}
	if err := untar(tarPath, dest); err != nil {
		os.RemoveAll(dest)
		return "", err
	}
	return dest, nil
}

// downloadArtifact is the one unimplemented seam. See fetchAndUnpack's TODO.
func downloadArtifact(e *index.PluginIndexEntry, plat *index.Platform) (string, error) {
	return "", fmt.Errorf(
		"feed download not yet implemented: would pull %s@%s file %s from Azure feed %s/%s\n"+
			"       (for now, build locally and run: dongle plugin install <dir>)",
		e.Feed.PackageName, e.Version, plat.File, e.Feed.Organization, e.Feed.Feed)
}

func installFromDir(hostVersion, protocol, srcDir string) int {
	m, err := manifest.Load(filepath.Join(srcDir, "plugin.json"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}

	if ok, reason, err := compat.Check(hostVersion, protocol, m.Requires); err != nil {
		fmt.Fprintln(os.Stderr, "error: bad constraint in manifest:", err)
		return 1
	} else if !ok {
		fmt.Fprintf(os.Stderr, "error: %s %s — not installing\n", m.Name, reason)
		return 1
	}

	dstDir := state.PluginVersionDir(m.Name, m.Version)
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	if err := copyFile(filepath.Join(srcDir, "plugin.json"), filepath.Join(dstDir, "plugin.json"), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	if err := copyFile(filepath.Join(srcDir, m.Entrypoint), filepath.Join(dstDir, m.Entrypoint), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}

	st, err := state.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	st.Plugins[m.Name] = state.Installed{Name: m.Name, ActiveVersion: m.Version}
	if err := st.Save(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}

	fmt.Printf("installed %s %s (command: dongle %s)\n", m.Name, m.Version, m.Name)
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
