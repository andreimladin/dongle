// Command resolve is a build-time-only helper — NOT a dongle subcommand and
// not part of the host↔plugin contract. scripts/build-binaries.sh shells out
// to it, once per (plugin, target platform), to read a plugin's Azure
// Artifacts feed coordinates straight out of a local index checkout, so the
// release script never hardcodes anything about the feed or reimplements
// the index's YAML parsing. It reuses internal/index's Manifest type and
// internal/index.LoadFile — nothing about manifest parsing lives here.
//
// Usage:
//
//	go run ./tools/resolve <name> --version <v> --os <os> --arch <arch> \
//	   --index <path> [--format env|json]
//
// It is read-only: it resolves and prints coordinates, and downloads or
// installs nothing.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	name := args[0]

	fs := flag.NewFlagSet("resolve", flag.ContinueOnError)
	version := fs.String("version", "", "plugin version (required)")
	goos := fs.String("os", "", "target GOOS (required)")
	goarch := fs.String("arch", "", "target GOARCH (required)")
	indexDir := fs.String("index", "", "local index checkout to read plugins/<name>.yaml from (required)")
	format := fs.String("format", "env", "output format: env or json")
	fs.Usage = printUsage
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	if *version == "" || *goos == "" || *goarch == "" || *indexDir == "" {
		fmt.Fprintln(os.Stderr, "error: --version, --os, --arch, and --index are all required")
		printUsage()
		return 2
	}

	manifestPath := filepath.Join(*indexDir, "plugins", name+".yaml")
	m, err := index.LoadFile(manifestPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}

	plat, err := platformFor(m, *goos, *goarch)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}

	bareVersion := strings.TrimPrefix(*version, "v") // upack versions are bare semver

	switch *format {
	case "env":
		fmt.Printf("ORG=%q\n", m.Feed.Organization)
		fmt.Printf("FEED=%q\n", m.Feed.Feed)
		fmt.Printf("PROJECT=%q\n", m.Feed.Project)
		fmt.Printf("PACKAGE=%q\n", plat.Package)
		fmt.Printf("VERSION=%q\n", bareVersion)
	case "json":
		b, err := json.MarshalIndent(map[string]string{
			"org":     m.Feed.Organization,
			"feed":    m.Feed.Feed,
			"project": m.Feed.Project,
			"package": plat.Package,
			"version": bareVersion,
		}, "", "  ")
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return 1
		}
		fmt.Println(string(b))
	default:
		fmt.Fprintf(os.Stderr, "error: unknown --format %q (want env or json)\n", *format)
		return 2
	}
	return 0
}

// platformFor selects the manifest's platform entry for an arbitrary
// os/arch — deliberately not runtime.GOOS/GOARCH, since the entire point of
// this tool is resolving coordinates for platforms other than the one it
// happens to be running on.
func platformFor(m *index.Manifest, goos, goarch string) (*index.Platform, error) {
	for i := range m.Platforms {
		if m.Platforms[i].Selector.OS == goos && m.Platforms[i].Selector.Arch == goarch {
			return &m.Platforms[i], nil
		}
	}
	return nil, fmt.Errorf("%s %s has no build for %s/%s", m.Name, m.Version, goos, goarch)
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "usage: resolve <name> --version <v> --os <os> --arch <arch> --index <path> [--format env|json]")
}
