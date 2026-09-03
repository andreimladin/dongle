// Package index manages the embedded git-based plugin catalog: a local clone,
// refreshed on a TTL, mapping plugin name -> version -> Azure feed artifact.
//
// The index URL and branch are build-time build inputs — baked in via
// -ldflags at build time (see cmd/buildinfo.go and configs/index.env) and
// wired into this package once at startup via SetDefaults. DONGLE_INDEX_URL
// and DONGLE_INDEX_BRANCH override them as dev escape hatches; normal users
// never set them.
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

// defaultURL and defaultBranch are the catalog git repository coordinates —
// a private Azure DevOps repo served over HTTPS — wired in once via
// SetDefaults from the build-time-injected values (cmd/buildinfo.go). dongle
// does not manage credentials for it — cloning and pulling shell out to the
// system `git`, which authenticates via Git Credential Manager (or whatever
// credential helper the machine already has configured).
var (
	defaultURL    string
	defaultBranch string
)

// SetDefaults wires the build-time-injected index URL/branch into this
// package. Called once at startup (see cmd/root.go's Execute), before any
// index/plugin command runs.
func SetDefaults(url, branch string) {
	defaultURL = url
	defaultBranch = branch
}

func indexURL() string {
	if v := os.Getenv("DONGLE_INDEX_URL"); v != "" {
		return v
	}
	return defaultURL
}

func indexBranch() string {
	if v := os.Getenv("DONGLE_INDEX_BRANCH"); v != "" {
		return v
	}
	return defaultBranch
}

func cacheDir() string { return filepath.Join(state.DataDir(), "index") }
func metaPath() string { return filepath.Join(state.DataDir(), "index.meta") }

var ErrNotFound = errors.New("plugin not found in index")

// --- index-side manifest types (YAML) ----------------------------------------

// Manifest is a plugin's manifest: the plugins/<name>.yaml file a plugin
// creator authors and PRs into the index repo.
type Manifest struct {
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
	Project      string `yaml:"project"` // set when the feed is project-scoped
	Feed         string `yaml:"feed"`
	PackageType  string `yaml:"packageType"` // e.g. "upack"
	PackageName  string `yaml:"packageName"`
}

type Platform struct {
	Selector Selector `yaml:"selector"`
	Package  string   `yaml:"package"` // Universal Package name to download for this os/arch (az --name)
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

// Status reports where the index points, which branch it's pinned to, and how
// old the cache is.
func Status() (url string, branch string, age time.Duration, cloned bool) {
	if _, err := os.Stat(cacheDir()); os.IsNotExist(err) {
		return indexURL(), indexBranch(), 0, false
	}
	age, _ = cacheAge()
	return indexURL(), indexBranch(), age, true
}

func clone() error {
	if err := os.MkdirAll(state.DataDir(), 0o755); err != nil {
		return err
	}
	if err := run("git", "clone", "--depth", "1", "--branch", indexBranch(),
		indexURL(), cacheDir()); err != nil {
		return fmt.Errorf("clone index: %w", err)
	}
	return touchMeta()
}

func pull() error {
	if err := run("git", "-C", cacheDir(), "pull", "--ff-only", "origin", indexBranch()); err != nil {
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

// Load reads and parses the manifest plugins/<name>.yaml from the local cache.
func Load(name string) (*Manifest, error) {
	path := filepath.Join(cacheDir(), "plugins", name+".yaml")
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := yaml.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("invalid manifest for %s: %w", name, err)
	}
	return &m, nil
}

// LoadFile reads and parses a manifest from an explicit file path, bypassing
// the cache (cacheDir/EnsureFresh/Refresh) entirely. Purely additive next to
// Load: used by build-time tools (tools/resolve) that need to read
// plugins/<name>.yaml out of an arbitrary index checkout — e.g. one just
// cloned fresh into a temp dir — without touching the managed cache.
func LoadFile(path string) (*Manifest, error) {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := yaml.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("invalid manifest at %s: %w", path, err)
	}
	return &m, nil
}

// List returns every plugin manifest in the cache (for `dongle plugin search`).
func List() ([]Manifest, error) {
	dir := filepath.Join(cacheDir(), "plugins")
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []Manifest
	for _, f := range files {
		if f.IsDir() || filepath.Ext(f.Name()) != ".yaml" {
			continue
		}
		if m, err := Load(stem(f.Name())); err == nil {
			out = append(out, *m)
		}
	}
	return out, nil
}

// PlatformFor returns the artifact matching the running os/arch.
func (m *Manifest) PlatformFor() (*Platform, error) {
	for i := range m.Platforms {
		if m.Platforms[i].Selector.OS == runtime.GOOS &&
			m.Platforms[i].Selector.Arch == runtime.GOARCH {
			return &m.Platforms[i], nil
		}
	}
	return nil, fmt.Errorf("%s %s has no build for %s/%s",
		m.Name, m.Version, runtime.GOOS, runtime.GOARCH)
}

func stem(fname string) string { return fname[:len(fname)-len(filepath.Ext(fname))] }
