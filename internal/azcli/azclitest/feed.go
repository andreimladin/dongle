// Package azclitest provides Feed, a fake Azure CLI (azcli.Runner) for
// tests, in the spirit of net/http/httptest: it answers the `az` commands
// dongle runs as if they reached an Azure Artifacts feed, without running
// anything. It is imported only from _test.go files; production code never
// imports it, so it is never linked into the dongle binary.
//
// The commands it understands:
//   - `az devops invoke ... packageNameQuery=<index>`: the latest index
//     version query (internal/index.LatestVersion), answered with Latest;
//   - `az artifacts universal download --name <index> --version <v>`: an
//     index download, which writes Indexes[v] as index.tar.gz into --path;
//   - `az artifacts universal download --name <other>`: a plugin download,
//     which writes a small fake plugin binary into --path.
package azclitest

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/andreimladin/dongle/internal/azcli"
)

// Feed is a fake azcli.Runner backed by an in-memory feed. Its fields
// configure what the feed holds and how it fails; Calls records every
// command it was asked to run.
type Feed struct {
	// IndexPackage is the index's package name: downloads of it are index
	// downloads, anything else is a plugin download.
	IndexPackage string
	// Indexes maps an index version ("*" for latest) to its archive.
	Indexes map[string][]byte
	// Latest is what the version query reports; "" means no index is
	// published, so the query fails with "not found".
	Latest string

	Missing      bool            // az isn't installed: LookPath fails
	QueryErr     error           // the version query fails (as unauthorized)
	DownloadErr  error           // every download fails (as not found)
	FailPackages map[string]bool // downloads of these packages fail (as unauthorized)
	ExtraFile    bool            // downloads drop a second file next to the package's

	Calls [][]string
}

// New returns an empty feed whose index package is indexPackage.
func New(indexPackage string) *Feed {
	return &Feed{IndexPackage: indexPackage, Indexes: map[string][]byte{}, FailPackages: map[string]bool{}}
}

// Install makes f the Azure CLI every package uses (azcli.Default) for the
// rest of the test, restoring the real one when the test ends.
func Install(t testing.TB, f *Feed) {
	t.Helper()
	orig := azcli.Default
	azcli.Default = f
	t.Cleanup(func() { azcli.Default = orig })
}

// Publish puts an index archive on the feed as version, which becomes the
// latest (also served for "*").
func (f *Feed) Publish(version string, archive []byte) {
	f.Indexes[version] = archive
	f.Indexes["*"] = archive
	f.Latest = version
}

func (f *Feed) LookPath() error {
	if f.Missing {
		return errors.New(`exec: "az": executable file not found in $PATH`)
	}
	return nil
}

func (f *Feed) Run(args []string, stdout, stderr io.Writer) error {
	f.Calls = append(f.Calls, args)
	switch {
	case isQuery(args):
		if f.QueryErr != nil {
			fmt.Fprint(stderr, "ERROR: unauthorized")
			return f.QueryErr
		}
		if f.Latest == "" {
			fmt.Fprintf(stderr, "ERROR: package %s not found", f.IndexPackage)
			return errors.New("exit status 1")
		}
		fmt.Fprintf(stdout, `{"value":[{"name":%q,"versions":[{"version":%q,"isLatest":true}]}]}`, f.IndexPackage, f.Latest)
		return nil
	case isDownload(args):
		return f.download(args, stderr)
	}
	return fmt.Errorf("azclitest: unexpected az %v", args)
}

func (f *Feed) download(args []string, stderr io.Writer) error {
	name, version, dest := FlagValue(args, "--name"), FlagValue(args, "--version"), FlagValue(args, "--path")
	switch {
	case f.DownloadErr != nil:
		fmt.Fprintf(stderr, "ERROR: package %s not found", name)
		return f.DownloadErr
	case f.FailPackages[name]:
		fmt.Fprint(stderr, "ERROR: unauthorized")
		return errors.New("exit status 1")
	}
	if name == f.IndexPackage {
		archive, ok := f.Indexes[version]
		if !ok {
			fmt.Fprintf(stderr, "ERROR: package %s not found", name)
			return errors.New("exit status 1")
		}
		if err := os.WriteFile(filepath.Join(dest, "index.tar.gz"), archive, 0o644); err != nil {
			return err
		}
	} else {
		// Named unpredictably, as a real package's file may be.
		body := fmt.Sprintf("#!/bin/sh\necho %s %s\n", name, version)
		if err := os.WriteFile(filepath.Join(dest, name+"-pkg.bin"), []byte(body), 0o644); err != nil {
			return err
		}
	}
	if f.ExtraFile {
		return os.WriteFile(filepath.Join(dest, "README"), nil, 0o644)
	}
	return nil
}

// Queries is how many latest-version queries the feed answered.
func (f *Feed) Queries() int {
	n := 0
	for _, c := range f.Calls {
		if isQuery(c) {
			n++
		}
	}
	return n
}

// IndexDownloads returns the --version of every index download, in order.
func (f *Feed) IndexDownloads() []string {
	var out []string
	for _, c := range f.Calls {
		if isDownload(c) && FlagValue(c, "--name") == f.IndexPackage {
			out = append(out, FlagValue(c, "--version"))
		}
	}
	return out
}

// PluginCalls returns every plugin download command, in order.
func (f *Feed) PluginCalls() [][]string {
	var out [][]string
	for _, c := range f.Calls {
		if isDownload(c) && FlagValue(c, "--name") != f.IndexPackage {
			out = append(out, c)
		}
	}
	return out
}

// PluginDownloads returns the --name of every plugin download, in order.
func (f *Feed) PluginDownloads() []string {
	var out []string
	for _, c := range f.PluginCalls() {
		out = append(out, FlagValue(c, "--name"))
	}
	return out
}

// FlagValue returns the value following flag in args, or "".
func FlagValue(args []string, flag string) string {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == flag {
			return args[i+1]
		}
	}
	return ""
}

func isQuery(args []string) bool { return len(args) > 1 && args[0] == "devops" && args[1] == "invoke" }

func isDownload(args []string) bool {
	return len(args) > 2 && args[0] == "artifacts" && args[1] == "universal" && args[2] == "download"
}
