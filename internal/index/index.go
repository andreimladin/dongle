// Package index manages the plugin catalog: a versioned "dongle-index"
// Universal Package (a tar+gzip archive of plugins/*.yaml manifests)
// downloaded from Azure Artifacts, extracted, and refreshed on a TTL,
// mapping plugin name -> version -> Azure feed artifact.
//
// A binary built with -tags embed also carries a seed copy of that same
// archive baked in at build time (see internal/bootstrap.EmbeddedIndex and
// scripts/build.sh's fetch_embedded). EnsureFresh extracts that seed into
// the cache on a first run with nothing cached yet, so a released dongle
// has a working index from its very first run, fully offline; a plain
// `go build ./cmd` embeds nothing, so that path falls back to a feed
// download as before. CachedOrigin reports which kind — embedded seed or
// feed-fetched — is currently in the cache.
//
// The feed package identity (org/project/feed/package name) is a
// build-time build input — baked in via -ldflags at build time (see
// cmd/root.go and configs/build.yaml) and wired into this package once at
// startup via SetDefaults. DONGLE_INDEX_ORG / DONGLE_INDEX_PROJECT /
// DONGLE_INDEX_FEED / DONGLE_INDEX_PACKAGE override them as dev escape
// hatches; normal users never set them.
package index

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/andreimladin/dongle/internal/bootstrap"
	"github.com/andreimladin/dongle/internal/compat"
	"github.com/andreimladin/dongle/internal/state"
	"github.com/andreimladin/dongle/internal/ui"
)

// injected holds the index package's Azure Artifacts feed coordinates —
// wired in once via SetDefaults from the build-time-injected values
// (cmd/root.go is the single source of these). Named fields rather than
// package-level vars so they don't collide with the indexOrg()/etc.
// accessors below. Downloading shells out to the system `az` CLI, which
// authenticates however it's already logged in (or AZURE_DEVOPS_EXT_PAT in
// CI).
var injected struct {
	indexOrg     string
	indexProject string
	indexFeed    string
	indexPackage string
}

// SetDefaults wires the build-time-injected index feed identity into this
// package. Called once at startup (see cmd/root.go's Execute), before any
// refresh/plugin command runs.
func SetDefaults(org, project, feed, pkg string) {
	injected.indexOrg = org
	injected.indexProject = project
	injected.indexFeed = feed
	injected.indexPackage = pkg
}

func indexOrg() string {
	if v := os.Getenv("DONGLE_INDEX_ORG"); v != "" {
		return v
	}
	return injected.indexOrg
}

func indexProject() string {
	if v := os.Getenv("DONGLE_INDEX_PROJECT"); v != "" {
		return v
	}
	return injected.indexProject
}

func indexFeed() string {
	if v := os.Getenv("DONGLE_INDEX_FEED"); v != "" {
		return v
	}
	return injected.indexFeed
}

func indexPackage() string {
	if v := os.Getenv("DONGLE_INDEX_PACKAGE"); v != "" {
		return v
	}
	return injected.indexPackage
}

// cacheDir is the extracted archive's root: it holds a "plugins/" subdir
// of manifests, mirroring index.tar.gz's own layout (see
// azure-pipelines-publish-index.yml in the index repo).
func cacheDir() string    { return filepath.Join(state.DataDir(), "index") }
func metaPath() string    { return filepath.Join(state.DataDir(), "index.meta") }
func versionPath() string { return filepath.Join(state.DataDir(), "index.version") }
func originPath() string  { return filepath.Join(state.DataDir(), "index.origin") }

// Origin values recorded in index.origin alongside the cache, so `dongle
// version` and callers can tell whether the index in use is still the seed
// baked into this binary at build time or one actually fetched from the
// feed.
const (
	OriginEmbedded = "embedded"
	OriginFetched  = "fetched"
)

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
	Support          Support         `yaml:"support"`
}

// Support is where users go for help with a plugin: shown by `dongle
// support <plugin>`. Documentation and Channel are mandatory (enforced by
// tools/validate-manifest); Contact is optional.
type Support struct {
	Documentation string `yaml:"documentation"`
	Channel       string `yaml:"channel"`
	Contact       string `yaml:"contact,omitempty"`
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

// EnsureFresh makes sure a usable index is cached locally before a command
// reads it, following this precedence:
//
//   - a fresh cache (within ttl) is used as-is, no feed call;
//   - no cache at all seeds one from the index archive embedded into this
//     binary at build time (see internal/bootstrap.EmbeddedIndex and
//     scripts/build.sh's fetch_embedded) — offline, no feed call — falling
//     back to a feed download only when nothing was embedded (a plain,
//     non -tags-embed build);
//   - a stale cache triggers a feed download to replace it; on failure the
//     existing cache is kept (a command running on slightly old data beats
//     one that can't run at all), so being offline never blocks commands
//     that can run on a slightly stale catalog. That case returns a
//     *StaleError, which callers report as a warning, not a failure.
func EnsureFresh(ttl time.Duration) error {
	if _, err := os.Stat(cacheDir()); os.IsNotExist(err) {
		if seedFromEmbedded() {
			return nil
		}
		return download()
	}
	age, err := cacheAge()
	if err != nil || age > ttl {
		if err := download(); err != nil {
			return &StaleError{Err: err}
		}
	}
	return nil
}

// StaleError is EnsureFresh's "refresh failed, but the (stale) cached
// index is still usable" result: a warning for the caller to show, not a
// reason to stop.
type StaleError struct{ Err error }

func (e *StaleError) Error() string {
	return fmt.Sprintf("could not refresh the plugin index (%v); using the cached copy", e.Err)
}

func (e *StaleError) Unwrap() error { return e.Err }

// NeedsRefresh reports whether EnsureFresh(ttl) would contact the feed —
// no cache and nothing embedded to seed it from, or a cache older than
// ttl — so callers can show progress only when there will be some.
func NeedsRefresh(ttl time.Duration) bool {
	if !HasCache() {
		return !HasEmbedded()
	}
	age, err := cacheAge()
	return err != nil || age > ttl
}

// HasEmbedded reports whether this binary carries a seed index (built with
// -tags embed and a staged index archive).
func HasEmbedded() bool {
	_, _, ok := bootstrap.EmbeddedIndex()
	return ok
}

// HasCache reports whether any index is cached locally.
func HasCache() bool {
	_, err := os.Stat(cacheDir())
	return err == nil
}

// EnsureCache makes sure *some* index is cached, without regard to its
// age: when nothing is cached yet it seeds the cache from the embedded
// index (offline), falling back to a feed download only when nothing was
// embedded. Commands that check the feed for a newer index themselves
// (install/upgrade) use this instead of EnsureFresh, so a stale cache is
// never silently replaced behind the user's back.
func EnsureCache() error {
	if HasCache() {
		return nil
	}
	if seedFromEmbedded() {
		return nil
	}
	return download()
}

// SeedEmbedded seeds the cache from the index embedded in this binary,
// replacing whatever is cached, without printing anything (callers may be
// showing a spinner). ok is false when nothing was embedded (a plain,
// non -tags-embed build); err reports a failed extraction.
func SeedEmbedded() (ok bool, err error) { return extractEmbedded() }

// seedFromEmbedded extracts the index archive baked into this binary (see
// internal/bootstrap.EmbeddedIndex and scripts/build.sh's fetch_embedded)
// into the cache, so a released binary has a working index from its very
// first run, fully offline. Returns false — leaving the cache untouched —
// when nothing was embedded (a plain, non -tags-embed build), so the
// caller can fall back to a feed download.
func seedFromEmbedded() bool {
	ok, err := extractEmbedded()
	if err != nil {
		ui.Warnf("could not seed the embedded plugin index: %v", err)
	}
	return ok && err == nil
}

// extractEmbedded does seedFromEmbedded's work without printing: ok is
// false when nothing was embedded; err reports a failed extraction, which
// leaves the cache untouched (except for a failure recording its version).
// Failing to record the origin/freshness metadata is not fatal and is
// ignored: the cache then just reads as fetched / stale.
func extractEmbedded() (ok bool, err error) {
	archive, version, embedded := bootstrap.EmbeddedIndex()
	if !embedded {
		return false, nil
	}

	stagingRoot := filepath.Join(state.DataDir(), ".staging")
	if err := os.MkdirAll(stagingRoot, 0o755); err != nil {
		return true, err
	}
	staging, err := os.MkdirTemp(stagingRoot, "index-embedded-")
	if err != nil {
		return true, err
	}
	defer os.RemoveAll(staging)

	extractDir := filepath.Join(staging, "extracted")
	if err := extractTarGzBytes(archive, extractDir); err != nil {
		return true, err
	}

	if err := os.RemoveAll(cacheDir()); err != nil {
		return true, err
	}
	if err := os.Rename(extractDir, cacheDir()); err != nil {
		return true, err
	}

	if err := os.WriteFile(versionPath(), []byte(version), 0o644); err != nil {
		return true, err
	}
	_ = writeOrigin(OriginEmbedded)
	_ = touchMeta()
	return true, nil
}

// CachedVersion returns the version of the index currently cached (read
// from the VERSION file recorded alongside the extracted manifests at
// download time), and whether an index has been cached at all.
func CachedVersion() (version string, ok bool) {
	b, err := os.ReadFile(versionPath())
	if err != nil {
		return "", false
	}
	v := strings.TrimSpace(string(b))
	if v == "" {
		return "", false
	}
	return v, true
}

// CachedOrigin reports whether the currently cached index is the embedded
// seed baked into this binary (OriginEmbedded) or one fetched from the
// feed (OriginFetched) — including a cache first seeded from the embedded
// copy and later replaced by a successful refresh. ok is false when no
// index is cached at all. A cache with no index.origin file (one written
// before origin tracking existed) is treated as OriginFetched, since every
// such cache could only ever have come from the feed.
func CachedOrigin() (origin string, ok bool) {
	if _, cok := CachedVersion(); !cok {
		return "", false
	}
	b, err := os.ReadFile(originPath())
	if err != nil {
		return OriginFetched, true
	}
	if strings.TrimSpace(string(b)) != OriginEmbedded {
		return OriginFetched, true
	}
	return OriginEmbedded, true
}

func writeOrigin(origin string) error {
	return os.WriteFile(originPath(), []byte(origin), 0o644)
}

// download fetches the latest version of the index package from the feed
// and swaps it in as the new cache.
func download() error {
	latest, err := FetchLatest()
	if err != nil {
		return err
	}
	return latest.Apply()
}

// Latest is the latest index package downloaded from the feed and
// extracted into a staging dir, but not yet installed as the cache — so a
// caller can compare its version against the cached one and decide
// whether to Apply it or Discard it.
type Latest struct {
	Version string // from the archive's VERSION file; empty if it had none

	staging    string
	extractDir string
}

// FetchLatest downloads the latest version of the index package from the
// feed and extracts it into a staging dir under the data dir (same
// filesystem, so Apply's swap is a rename). The caller must Apply or
// Discard the result.
func FetchLatest() (*Latest, error) {
	if _, err := exec.LookPath("az"); err != nil {
		return nil, fmt.Errorf("the Azure CLI is required: install it and run `az extension add --name azure-devops`")
	}
	stagingRoot := filepath.Join(state.DataDir(), ".staging")
	if err := os.MkdirAll(stagingRoot, 0o755); err != nil {
		return nil, err
	}
	staging, err := os.MkdirTemp(stagingRoot, "index-")
	if err != nil {
		return nil, err
	}
	l := &Latest{staging: staging, extractDir: filepath.Join(staging, "extracted")}

	archivePath, err := downloadArchive(filepath.Join(staging, "download"))
	if err != nil {
		l.Discard()
		return nil, err
	}
	if err := extractTarGz(archivePath, l.extractDir); err != nil {
		l.Discard()
		return nil, fmt.Errorf("extract index archive: %w", err)
	}
	if b, err := os.ReadFile(filepath.Join(l.extractDir, "VERSION")); err == nil {
		l.Version = strings.TrimSpace(string(b))
	}
	return l, nil
}

// Apply installs the fetched index as the cache (atomically replacing the
// previous one), records its version and origin, and marks the cache
// fresh.
func (l *Latest) Apply() error {
	defer l.Discard()
	if err := os.RemoveAll(cacheDir()); err != nil {
		return err
	}
	if err := os.Rename(l.extractDir, cacheDir()); err != nil {
		return fmt.Errorf("install extracted index: %w", err)
	}
	if l.Version != "" {
		if err := os.WriteFile(versionPath(), []byte(l.Version), 0o644); err != nil {
			return err
		}
	}
	if err := writeOrigin(OriginFetched); err != nil {
		return err
	}
	return touchMeta()
}

// Discard removes the fetched index's staging dir without installing it.
// Safe to call more than once.
func (l *Latest) Discard() { os.RemoveAll(l.staging) }

// MarkChecked records that the cached index was just confirmed current
// against the feed, restarting its TTL without re-downloading it.
func MarkChecked() error { return touchMeta() }

// IsNewer reports whether index version a is newer than b. Index versions
// are dotted numbers (e.g. 2024.03.01.1), compared part by part
// numerically; a non-numeric part falls back to a string comparison, and
// an empty b (no version recorded) is older than anything.
func IsNewer(a, b string) bool {
	a = strings.TrimPrefix(strings.TrimSpace(a), "v")
	b = strings.TrimPrefix(strings.TrimSpace(b), "v")
	if b == "" {
		return a != ""
	}
	ap, bp := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(ap) && i < len(bp); i++ {
		an, aerr := strconv.Atoi(ap[i])
		bn, berr := strconv.Atoi(bp[i])
		switch {
		case aerr == nil && berr == nil:
			if an != bn {
				return an > bn
			}
		case ap[i] != bp[i]:
			return ap[i] > bp[i]
		}
	}
	return len(ap) > len(bp)
}

// downloadArchive shells out to the Azure CLI to pull the latest version
// of the index Universal Package into destDir, returning the path to the
// single downloaded file (expected to be index.tar.gz, but the package's
// contents aren't assumed to be named predictably — the same defensiveness
// internal/builtins.downloadArtifact uses).
func downloadArchive(destDir string) (string, error) {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", err
	}
	args := []string{
		"artifacts", "universal", "download",
		"--organization", "https://dev.azure.com/" + indexOrg(),
		"--feed", indexFeed(),
		"--name", indexPackage(),
		// "*" resolves to the latest published version — a feature specific
		// to Universal Packages (unlike other Azure Artifacts package
		// types). TODO: verify this exact flag behavior against the az CLI
		// / azure-devops extension version you deploy with; if it ever
		// changes, resolve the latest version explicitly (e.g. via the
		// Azure Artifacts feed API) before calling download with it.
		"--version", "*",
		"--path", destDir,
	}
	if indexProject() != "" {
		args = append(args, "--project", indexProject(), "--scope", "project")
	}
	// az's stderr is captured rather than inherited so it can't garble a
	// spinner; it's surfaced in the error if the download fails.
	var stderr bytes.Buffer
	cmd := exec.Command("az", args...)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", AzError("az download "+indexPackage(), err, stderr.Bytes())
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
		return "", fmt.Errorf("package %s contains %d files, expected exactly 1", indexPackage(), len(files))
	}
	return filepath.Join(destDir, files[0]), nil
}

// AzError wraps a failed `az` invocation's error with whatever it wrote
// to stderr, so the cause (auth, missing package, ...) isn't lost when az
// output is captured instead of shown live.
func AzError(what string, err error, stderr []byte) error {
	if msg := strings.TrimSpace(string(stderr)); msg != "" {
		return fmt.Errorf("%s: %w\n%s", what, err, msg)
	}
	return fmt.Errorf("%s: %w", what, err)
}

// extractTarGz extracts a tar+gzip archive file into destDir, which must
// not already exist.
func extractTarGz(archivePath, destDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()
	return extractTarGzReader(f, destDir)
}

// extractTarGzBytes is extractTarGzReader over an in-memory archive — the
// index archive embedded into the binary (see seedFromEmbedded) — rather
// than one just downloaded to disk.
func extractTarGzBytes(archive []byte, destDir string) error {
	return extractTarGzReader(bytes.NewReader(archive), destDir)
}

// extractTarGzReader extracts a tar+gzip stream into destDir, which must
// not already exist. Rejects entries that would escape destDir
// ("zip-slip").
func extractTarGzReader(r io.Reader, destDir string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gz.Close()

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	destRoot := filepath.Clean(destDir)

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(destRoot, hdr.Name)
		if target != destRoot && !strings.HasPrefix(target, destRoot+string(os.PathSeparator)) {
			return fmt.Errorf("archive entry escapes destination: %s", hdr.Name)
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
			out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(hdr.Mode)&0o777)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			if err := out.Close(); err != nil {
				return err
			}
		}
	}
	return nil
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
// Load: used by build-time tools (tools/resolve-plugin) that need to read
// plugins/<name>.yaml out of an arbitrary extracted index archive — e.g. one
// just downloaded fresh into a temp dir — without touching the managed cache.
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

// LoadFileStrict is LoadFile but rejects unknown YAML fields instead of
// silently ignoring them. Purely additive next to LoadFile/Load: used only
// by tools/validate-manifest, which needs to catch manifest typos (e.g. a
// misspelled field name) that would otherwise pass through unnoticed.
func LoadFileStrict(path string) (*Manifest, error) {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var m Manifest
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("invalid manifest at %s: %w", path, err)
	}
	return &m, nil
}

// List returns every plugin manifest in the cache (for `dongle search`).
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
