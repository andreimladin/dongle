// Command validate-manifest is a build-time-only helper — NOT a dongle
// subcommand and not part of the host↔plugin contract. It validates
// plugins/<name>.yaml manifests against the exact same types and parsing
// internal/index uses at install time, so validation can never drift from
// how the real CLI reads a manifest. It is meant to be published to the
// feed (see azure-pipelines-tooling.yml) and run from the index repo's own
// PR pipeline (see azure-pipelines-validate.yml, generated separately for
// that repo) before a manifest is merged.
//
// Usage:
//
//	validate-manifest <file>...            validate specific manifest files
//	validate-manifest --dir <plugins-dir>  scan every *.yaml in a directory
//
// Two layers are checked per manifest:
//
//  1. Correctness — strict YAML parsing (unknown fields rejected), name
//     matches the filename, version is valid semver, feed coordinates are
//     present, and every declared platform entry has both a selector and a
//     package reference. Missing platforms are fine — only declared entries
//     are checked, coverage/completeness is not this tool's job.
//  2. Existence — for each declared platform, confirm that platform's
//     package actually exists at the manifest's version in the feed the
//     manifest itself names (not a global/configured feed). See
//     existence.go: this shells out to `az artifacts universal download`
//     into a throwaway temp dir, same as dongle's own installer — a
//     successful download is the existence proof, and its content is
//     discarded immediately.
//
// Existence checks require the Azure CLI (az) to be present on PATH and
// already authenticated — either an existing `az login` session, or (as in
// CI) an AZURE_DEVOPS_EXT_PAT in the environment, which `az artifacts`
// picks up on its own. validate-manifest never calls `az login` itself.
//
// Exit codes: 0 all manifests valid, 1 one or more manifests failed
// validation (or existence couldn't be confirmed), 2 usage error.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/andreimladin/dongle/internal/compat"
	"github.com/andreimladin/dongle/internal/index"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		printUsage()
		if len(args) == 0 {
			return 2
		}
		return 0
	}

	var files []string
	if args[0] == "--dir" {
		if len(args) != 2 {
			fmt.Fprintln(os.Stderr, "error: --dir requires exactly one directory argument")
			printUsage()
			return 2
		}
		fs, err := manifestsInDir(args[1])
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return 1
		}
		files = fs
	} else {
		files = args
	}

	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "error: no manifest files to validate")
		return 1
	}

	azAvailable := azOnPath()
	if !azAvailable {
		fmt.Fprintln(os.Stderr, "warning: az CLI not found on PATH — package-existence checks will fail closed for every manifest")
	}

	passed := 0
	for _, f := range files {
		if validateFile(f, azAvailable) {
			passed++
		}
	}

	fmt.Printf("\n%d/%d manifests passed\n", passed, len(files))
	if passed != len(files) {
		return 1
	}
	return 0
}

// manifestsInDir lists every *.yaml directly under dir (non-recursive),
// sorted for stable, diffable output.
func manifestsInDir(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
			continue
		}
		files = append(files, filepath.Join(dir, e.Name()))
	}
	sort.Strings(files)
	return files, nil
}

// validateFile runs both layers for one manifest file and prints a
// per-file, per-check report — correctness as a whole, then one pass/fail
// line per declared platform's package. It returns false if the manifest
// failed either layer.
func validateFile(path string, azAvailable bool) bool {
	fmt.Printf("== %s ==\n", path)

	m, errs := checkCorrectness(path)
	if len(errs) > 0 {
		fmt.Println("  correctness: FAIL")
		for _, e := range errs {
			fmt.Printf("    error: %s\n", e)
		}
		return false
	}
	fmt.Println("  correctness: OK")

	results := checkExistence(m, azAvailable)
	ok := true
	for _, r := range results {
		if r.err != nil {
			ok = false
			fmt.Printf("  %s/%s %s@%s: FAIL — %v\n", r.os, r.arch, r.pkg, r.version, r.err)
			continue
		}
		fmt.Printf("  %s/%s %s@%s: OK\n", r.os, r.arch, r.pkg, r.version)
	}
	return ok
}

var semverPattern = regexp.MustCompile(
	`^v?(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(-[0-9A-Za-z-.]+)?(\+[0-9A-Za-z-.]+)?$`,
)

// checkCorrectness is layer (a): strict parse plus the mandatory-field
// checks. It reuses internal/index's Manifest type (via LoadFileStrict) and
// internal/compat's constraint parser so this tool's notion of "valid"
// cannot drift from what the real CLI accepts. m is nil when parsing itself
// failed; callers must not read it in that case (errs will be non-empty).
func checkCorrectness(path string) (m *index.Manifest, errs []string) {
	m, err := index.LoadFileStrict(path)
	if err != nil {
		return nil, []string{err.Error()}
	}

	base := filepath.Base(path)
	stem := strings.TrimSuffix(base, filepath.Ext(base))

	if m.Name == "" {
		errs = append(errs, "name is required")
	} else if m.Name != stem {
		errs = append(errs, fmt.Sprintf("name %q does not match filename %q (expected name: %s)", m.Name, base, stem))
	}

	if m.Version == "" {
		errs = append(errs, "version is required")
	} else if !semverPattern.MatchString(m.Version) {
		errs = append(errs, fmt.Sprintf("version %q is not a valid semver", m.Version))
	}

	if m.Requires.Host != "" {
		if _, err := compat.SatisfiesHost("0.0.0", m.Requires.Host); err != nil {
			errs = append(errs, fmt.Sprintf("requires.host %q: %v", m.Requires.Host, err))
		}
	}

	if m.Feed.Organization == "" {
		errs = append(errs, "feed.organization is required")
	}
	if m.Feed.Feed == "" {
		errs = append(errs, "feed.feed is required")
	}
	if m.Feed.PackageType == "" {
		errs = append(errs, "feed.packageType is required")
	}
	if m.Feed.PackageName == "" {
		errs = append(errs, "feed.packageName is required")
	}

	if len(m.Platforms) == 0 {
		errs = append(errs, "platforms: at least one platform entry is required")
	}
	for i, p := range m.Platforms {
		if p.Selector.OS == "" {
			errs = append(errs, fmt.Sprintf("platforms[%d].selector.os is required", i))
		}
		if p.Selector.Arch == "" {
			errs = append(errs, fmt.Sprintf("platforms[%d].selector.arch is required", i))
		}
		if p.Package == "" {
			errs = append(errs, fmt.Sprintf("platforms[%d].package is required", i))
		}
	}

	return m, errs
}

// packageCheck is one platform's existence-check result.
type packageCheck struct {
	os, arch, pkg, version string
	err                    error // nil means the package exists and is downloadable
}

// checkExistence is layer (b): for each declared platform, confirm that
// platform's package exists at m.Version in the feed m itself names. Only
// called once checkCorrectness has passed, so m's feed/platform fields are
// known to be populated. Always returns one result per platform (pass or
// fail) so callers can report a complete per-package breakdown.
func checkExistence(m *index.Manifest, azAvailable bool) []packageCheck {
	version := strings.TrimPrefix(m.Version, "v") // upack versions are bare semver
	results := make([]packageCheck, 0, len(m.Platforms))
	for _, p := range m.Platforms {
		r := packageCheck{os: p.Selector.OS, arch: p.Selector.Arch, pkg: p.Package, version: version}
		switch {
		case !azAvailable:
			r.err = fmt.Errorf("az CLI not found; cannot verify package existence (see: az extension add --name azure-devops)")
		default:
			r.err = packageExists(m.Feed, p.Package, version)
		}
		results = append(results, r)
	}
	return results
}

func azOnPath() bool {
	_, err := exec.LookPath("az")
	return err == nil
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "usage: validate-manifest <file>...")
	fmt.Fprintln(os.Stderr, "       validate-manifest --dir <plugins-dir>")
}
