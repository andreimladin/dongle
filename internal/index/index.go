// Package index manages the embedded git-based plugin catalog: a local clone,
// refreshed on a TTL, mapping plugin name -> version -> Azure feed artifact.
//
// The index URL is embedded (not user-editable). DONGLE_INDEX_URL overrides it
// as a dev escape hatch; normal users never set it.
package index

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/andreimladin/dongle/internal/compat"
	"github.com/andreimladin/dongle/internal/state"
)

// IndexURL is the catalog git repository. Put your Azure DevOps repo URL here,
// e.g. "https://dev.azure.com/acme/platform/_git/dongle-index".
const IndexURL = "PUT_YOUR_INDEX_GIT_URL_HERE"

func indexURL() string {
	if v := os.Getenv("DONGLE_INDEX_URL"); v != "" {
		return v
	}
	return IndexURL
}

func cacheDir() string { return filepath.Join(state.DataDir(), "index") }
func metaPath() string { return filepath.Join(state.DataDir(), "index.meta") }

var ErrNotFound = errors.New("plugin not found in index")

// --- index-side manifest types (YAML) ----------------------------------------

type PluginIndexEntry struct {
	Name             string          `yaml:"name"`
	Version          string          `yaml:"version"`
	ShortDescription string          `yaml:"shortDescription"`
	Requires         compat.Requires `yaml:"requires"`
	Feed             Feed            `yaml:"feed"`
	Platforms        []Platform      `yaml:"platforms"`
}

// Feed locates the artifact in Azure Artifacts (one Universal Package per plugin
// in a shared feed).
type Feed struct {
	Organization string `yaml:"organization"`
	Feed         string `yaml:"feed"`
	PackageType  string `yaml:"packageType"` // e.g. "upack"
	PackageName  string `yaml:"packageName"`
}

type Platform struct {
	Selector Selector `yaml:"selector"`
	File     string   `yaml:"file"`   // file within the package version
	SHA256   string   `yaml:"sha256"` // verified after download
	Bin      string   `yaml:"bin"`    // entrypoint filename inside the tarball
}

type Selector struct {
	OS   string `yaml:"os"`
	Arch string `yaml:"arch"`
}

// --- refresh lifecycle --------------------------------------------------------

// EnsureFresh clones the index if absent, or pulls if the cache is older than
// ttl. A pull failure with an existing cache is a non-fatal warning, so being
// offline never blocks commands that can run on a slightly stale catalog.
func EnsureFresh(ttl time.Duration) error {
	if _, err := os.Stat(cacheDir()); os.IsNotExist(err) {
		return clone()
	}
	age, err := cacheAge()
	if err != nil || age > ttl {
		if err := pull(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not refresh index (%v); using cached copy\n", err)
		}
	}
	return nil
}

// Refresh forces a pull now, ignoring the TTL (`dongle index refresh`).
func Refresh() error {
	if _, err := os.Stat(cacheDir()); os.IsNotExist(err) {
		return clone()
	}
	return pull()
}

// Status reports where the index points and how old the cache is.
func Status() (url string, age time.Duration, cloned bool) {
	if _, err := os.Stat(cacheDir()); os.IsNotExist(err) {
		return indexURL(), 0, false
	}
	age, _ = cacheAge()
	return indexURL(), age, true
}

func clone() error {
	if err := os.MkdirAll(state.DataDir(), 0o755); err != nil {
		return err
	}
	if err := run("git", "clone", "--depth", "1", indexURL(), cacheDir()); err != nil {
		return fmt.Errorf("clone index: %w", err)
	}
	return touchMeta()
}

func pull() error {
	if err := run("git", "-C", cacheDir(), "pull", "--ff-only"); err != nil {
		return fmt.Errorf("pull index: %w", err)
	}
	return touchMeta()
}

func cacheAge() (time.Duration, error) {
	fi, err := os.Stat(metaPath())
	if err != nil {
		return 0, err
	}
	return time.Since(fi.ModTime()), nil
}

func touchMeta() error {
	now := time.Now()
	if err := os.WriteFile(metaPath(), []byte(now.Format(time.RFC3339)), 0o644); err != nil {
		return err
	}
	return os.Chtimes(metaPath(), now, now)
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr // git progress -> stderr
	return cmd.Run()
}

// --- lookups ------------------------------------------------------------------

// Manifest reads plugins/<name>.yaml from the local cache.
func Manifest(name string) (*PluginIndexEntry, error) {
	path := filepath.Join(cacheDir(), "plugins", name+".yaml")
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var e PluginIndexEntry
	if err := yaml.Unmarshal(b, &e); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &e, nil
}

// List returns every plugin entry in the cache (for `dongle plugin search`).
func List() ([]PluginIndexEntry, error) {
	dir := filepath.Join(cacheDir(), "plugins")
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []PluginIndexEntry
	for _, f := range files {
		if f.IsDir() || filepath.Ext(f.Name()) != ".yaml" {
			continue
		}
		if e, err := Manifest(stem(f.Name())); err == nil {
			out = append(out, *e)
		}
	}
	return out, nil
}

// PlatformFor returns the artifact matching the running os/arch.
func (e *PluginIndexEntry) PlatformFor() (*Platform, error) {
	for i := range e.Platforms {
		if e.Platforms[i].Selector.OS == runtime.GOOS &&
			e.Platforms[i].Selector.Arch == runtime.GOARCH {
			return &e.Platforms[i], nil
		}
	}
	return nil, fmt.Errorf("%s %s has no build for %s/%s",
		e.Name, e.Version, runtime.GOOS, runtime.GOARCH)
}

func stem(fname string) string { return fname[:len(fname)-len(filepath.Ext(fname))] }
