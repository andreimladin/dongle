package builtins

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/andreimladin/dongle/internal/compat"
	"github.com/andreimladin/dongle/internal/index"
	"github.com/andreimladin/dongle/internal/state"
	"github.com/andreimladin/dongle/internal/ui"
)

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
