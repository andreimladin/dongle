// Command readconfig is a build-time-only helper — NOT a dongle subcommand
// and not part of the host↔plugin contract. scripts/build.sh shells out to
// it to read build.yaml (the single source of truth for the index
// coordinates and the embedded plugin list) and print it back out in
// whatever plain, shell-friendly shape the caller needs — so the script
// itself hardcodes none of it. It reuses gopkg.in/yaml.v3 (already a
// dependency, via internal/index) rather than pulling in a YAML CLI tool.
//
// Usage:
//
//	go run ./tools/readconfig --index [--file <path>]
//	go run ./tools/readconfig --embedded [--file <path>]
//
// --index prints INDEX_URL/INDEX_BRANCH as eval-able shell assignments.
// --embedded prints one "name:version" line per embedded plugin.
//
// It is read-only: it parses and prints, and downloads or builds nothing.
package main

import (
	"flag"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// buildConfig mirrors build.yaml's shape.
type buildConfig struct {
	Index struct {
		URL    string `yaml:"url"`
		Branch string `yaml:"branch"`
	} `yaml:"index"`
	Embedded []struct {
		Name    string `yaml:"name"`
		Version string `yaml:"version"`
	} `yaml:"embedded"`
}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet("readconfig", flag.ContinueOnError)
	index := fs.Bool("index", false, "print INDEX_URL/INDEX_BRANCH as shell assignments")
	embedded := fs.Bool("embedded", false, "print one name:version line per embedded plugin")
	configPath := fs.String("file", "build.yaml", "path to build.yaml")
	fs.Usage = printUsage
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *index == *embedded {
		fmt.Fprintln(os.Stderr, "error: exactly one of --index or --embedded is required")
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

	switch {
	case *index:
		fmt.Printf("INDEX_URL=%q\n", cfg.Index.URL)
		fmt.Printf("INDEX_BRANCH=%q\n", cfg.Index.Branch)
	case *embedded:
		for _, e := range cfg.Embedded {
			fmt.Printf("%s:%s\n", e.Name, e.Version)
		}
	}
	return 0
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "usage: readconfig (--index|--embedded) [--file path]")
}
