// Command readconfig is a build-time-only helper — NOT a dongle subcommand
// and not part of the host↔plugin contract. scripts/build.sh shells out to
// it to read configs/build.yaml (the single source of truth for the index
// feed identity and the embedded plugin list) and print it back out in
// whatever plain, shell-friendly shape the caller needs — so the script
// itself hardcodes none of it. It reuses gopkg.in/yaml.v3 (already a
// dependency, via internal/index) rather than pulling in a YAML CLI tool.
//
// Usage:
//
//	go run ./tools/readconfig --index [--file <path>]
//	go run ./tools/readconfig --embedded [--file <path>]
//
// --index prints INDEX_ORG/INDEX_PROJECT/INDEX_FEED/INDEX_PACKAGE as
// eval-able shell assignments. --embedded prints one plugin name per line
// (versions are no longer pinned here — see configs/build.yaml).
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
		Organization string `yaml:"organization"`
		Project      string `yaml:"project"`
		Feed         string `yaml:"feed"`
		Package      string `yaml:"package"`
	} `yaml:"index"`
	Embedded []string `yaml:"embedded"`
}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet("readconfig", flag.ContinueOnError)
	index := fs.Bool("index", false, "print INDEX_ORG/INDEX_PROJECT/INDEX_FEED/INDEX_PACKAGE as shell assignments")
	embedded := fs.Bool("embedded", false, "print one plugin name per line")
	configPath := fs.String("file", "configs/build.yaml", "path to build.yaml")
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
		fmt.Printf("INDEX_ORG=%q\n", cfg.Index.Organization)
		fmt.Printf("INDEX_PROJECT=%q\n", cfg.Index.Project)
		fmt.Printf("INDEX_FEED=%q\n", cfg.Index.Feed)
		fmt.Printf("INDEX_PACKAGE=%q\n", cfg.Index.Package)
	case *embedded:
		for _, name := range cfg.Embedded {
			fmt.Println(name)
		}
	}
	return 0
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "usage: readconfig (--index|--embedded) [--file path]")
}
