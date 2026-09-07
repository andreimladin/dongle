// Command resolve-plugin is a build-time-only helper — NOT a dongle
// subcommand and not part of the host↔plugin contract.
// scripts/build.sh's fetch_embedded shells out to it, once per embedded
// plugin, to read that plugin's Azure Artifacts feed coordinates straight
// out of a local index checkout, so the build script never hardcodes
// anything about the feed or reimplements the index's YAML parsing. It
// reuses internal/index's Manifest type and internal/index.LoadFile —
// nothing about manifest parsing lives here.
//
// Usage:
//
//	go run ./tools/resolve-plugin <name> <version> <os> <arch> --index <path>
//
// It is read-only: it resolves and prints coordinates, and downloads or
// installs nothing.
package main

import (
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
	if len(args) < 4 {
		fmt.Fprintln(os.Stderr, "error: name, version, os, and arch are all required")
		printUsage()
		return 2
	}
	name, version, goos, goarch := args[0], args[1], args[2], args[3]

	fs := flag.NewFlagSet("resolve-plugin", flag.ContinueOnError)
	indexDir := fs.String("index", "", "local index checkout to read plugins/<name>.yaml from (required)")
	fs.Usage = printUsage
	if err := fs.Parse(args[4:]); err != nil {
		return 2
	}
	if *indexDir == "" {
		fmt.Fprintln(os.Stderr, "error: --index is required")
		printUsage()
		return 2
	}

	manifestPath := filepath.Join(*indexDir, "plugins", name+".yaml")
	m, err := index.LoadFile(manifestPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}

	plat, err := platformFor(m, goos, goarch)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}

	bareVersion := strings.TrimPrefix(version, "v") // upack versions are bare semver

	fmt.Printf("ORG=%q\n", m.Feed.Organization)
	fmt.Printf("FEED=%q\n", m.Feed.Feed)
	fmt.Printf("PROJECT=%q\n", m.Feed.Project)
	fmt.Printf("PACKAGE=%q\n", plat.Package)
	fmt.Printf("VERSION=%q\n", bareVersion)
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
	fmt.Fprintln(os.Stderr, "usage: resolve-plugin <name> <version> <os> <arch> --index <path>")
}
