// Command buildconfig is a build-time-only helper — NOT a dongle subcommand
// and not part of the host↔plugin contract. scripts/build-binaries.sh shells
// out to it to read configs/build.yaml (the single source of truth for the
// index coordinates, the embedded plugin list, and the release target
// platforms) and print it back out in whatever plain, shell-friendly shape
// the script needs for that section — so the script itself hardcodes none
// of it and configs/build.yaml stays the one file to edit. It reuses
// gopkg.in/yaml.v3 (already a dependency, via internal/index) rather than
// pulling in a YAML CLI tool.
//
// Usage:
//
//	go run ./tools/buildconfig --get <index|targets|embedded> [--config <path>]
//
// It is read-only: it parses and prints, and downloads or builds nothing.
package main

import (
	"flag"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// buildConfig mirrors configs/build.yaml's shape.
type buildConfig struct {
	Index struct {
		URL    string `yaml:"url"`
		Branch string `yaml:"branch"`
	} `yaml:"index"`
	Embedded []struct {
		Name    string `yaml:"name"`
		Version string `yaml:"version"`
	} `yaml:"embedded"`
	Targets []struct {
		OS   string `yaml:"os"`
		Arch string `yaml:"arch"`
	} `yaml:"targets"`
}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet("buildconfig", flag.ContinueOnError)
	get := fs.String("get", "", "what to print: index, targets, or embedded (required)")
	configPath := fs.String("config", "configs/build.yaml", "path to build.yaml")
	fs.Usage = printUsage
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *get == "" {
		fmt.Fprintln(os.Stderr, "error: --get is required (index, targets, or embedded)")
		printUsage()
		return 2
	}

	b, err := os.ReadFile(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	var cfg buildConfig
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		fmt.Fprintf(os.Stderr, "error: invalid %s: %v\n", *configPath, err)
		return 1
	}

	switch *get {
	case "index":
		fmt.Printf("INDEX_URL=%q\n", cfg.Index.URL)
		fmt.Printf("INDEX_BRANCH=%q\n", cfg.Index.Branch)
	case "targets":
		for _, t := range cfg.Targets {
			fmt.Printf("%s %s\n", t.OS, t.Arch)
		}
	case "embedded":
		for _, e := range cfg.Embedded {
			fmt.Printf("%s %s\n", e.Name, e.Version)
		}
	default:
		fmt.Fprintf(os.Stderr, "error: unknown --get %q (want index, targets, or embedded)\n", *get)
		return 2
	}
	return 0
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "usage: buildconfig --get <index|targets|embedded> [--config path]")
}
