package builtins

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/andreimladin/dongle/internal/azcli"
	"github.com/andreimladin/dongle/internal/index"
	"github.com/andreimladin/dongle/internal/state"
	"github.com/andreimladin/dongle/internal/testutil"
)

// indexPackage is the index feed package every test's build "bakes in"
// (see newEnv).
const indexPackage = "dongle-index"

// fakeAz stands in for the Azure CLI (azcli.Default), which every feed
// access goes through:
//   - plugin downloads (`az artifacts universal download --name <pkg>`)
//     drop a plugin binary, named unpredictably as real packages may be,
//     into --path and are recorded in calls;
//   - index traffic — the latest-version query (`az devops invoke`) and
//     the index archive download — is served from the published index
//     (see publishIndex) and recorded in indexCalls.
type fakeAz struct {
	missing bool
	failFor map[string]bool // package names (plugin or index) whose download fails
	extra   bool            // plugin downloads also drop a second file
	calls   [][]string      // plugin downloads

	index        []byte // published index archive; nil: no index on the feed
	indexVersion string
	indexCalls   [][]string
}

func (f *fakeAz) LookPath() error {
	if f.missing {
		return errors.New("not found")
	}
	return nil
}

func (f *fakeAz) Run(args []string, stdout, stderr io.Writer) error {
	name, version, dest := flagValue(args, "--name"), flagValue(args, "--version"), flagValue(args, "--path")
	switch {
	case args[0] == "devops": // latest index version query
		f.indexCalls = append(f.indexCalls, args)
		if f.index == nil {
			fmt.Fprint(stderr, "ERROR: package "+indexPackage+" not found")
			return errors.New("exit status 1")
		}
		fmt.Fprintf(stdout, `{"value":[{"name":%q,"versions":[{"version":%q,"isLatest":true}]}]}`, indexPackage, f.indexVersion)
		return nil
	case name == indexPackage: // index archive download
		f.indexCalls = append(f.indexCalls, args)
		if f.index == nil || f.failFor[name] {
			fmt.Fprint(stderr, "ERROR: package "+indexPackage+" not found")
			return errors.New("exit status 1")
		}
		return os.WriteFile(filepath.Join(dest, "index.tar.gz"), f.index, 0o644)
	}
	f.calls = append(f.calls, args)
	if f.failFor[name] {
		fmt.Fprint(stderr, "ERROR: unauthorized")
		return errors.New("exit status 1")
	}
	body := fmt.Sprintf("#!/bin/sh\necho %s %s\n", name, version)
	if err := os.WriteFile(filepath.Join(dest, name+"-pkg.bin"), []byte(body), 0o644); err != nil {
		return err
	}
	if f.extra {
		return os.WriteFile(filepath.Join(dest, "LICENSE"), nil, 0o644)
	}
	return nil
}

func flagValue(args []string, flag string) string {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == flag {
			return args[i+1]
		}
	}
	return ""
}

// downloadedPackages returns the --name of every plugin download, in order.
func (f *fakeAz) downloadedPackages() []string {
	var out []string
	for _, c := range f.calls {
		out = append(out, flagValue(c, "--name"))
	}
	return out
}

// publishIndex puts an index archive on the fake feed: its VERSION, its
// defaults.yaml (omitted when defaults is nil) and a manifest per plugin.
func (f *fakeAz) publishIndex(t *testing.T, version string, defaults []string, plugins ...plugin) {
	t.Helper()
	files := map[string]string{"VERSION": version + "\n"}
	if defaults != nil {
		files[index.DefaultsFile] = "defaults:\n"
		for _, d := range defaults {
			files[index.DefaultsFile] += "  - " + d + "\n"
		}
	}
	for _, p := range plugins {
		files["plugins/"+p.name+".yaml"] = p.yaml()
	}
	f.index, f.indexVersion = tarGz(t, files), version
}

func tarGz(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, body := range files {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// newEnv isolates the test (temp HOME so ~/.dongle is a temp dir, empty
// PATH) and installs a fake az with an index feed identity "baked in"; the
// feed has no index until the test publishes one.
func newEnv(t *testing.T) *fakeAz {
	t.Helper()
	testutil.Isolate(t)
	f := &fakeAz{failFor: map[string]bool{}}
	orig := azcli.Default
	azcli.Default = f
	index.SetDefaults("acme", "", "plugins", indexPackage)
	t.Cleanup(func() {
		azcli.Default = orig
		index.SetDefaults("", "", "", "")
	})
	return f
}

// plugin is a test manifest; render writes it as index YAML.
type plugin struct {
	name, version, host, protocol, project string
	noPlatform                             bool
}

func (p plugin) yaml() string {
	var b strings.Builder
	fmt.Fprintf(&b, "name: %s\nversion: %s\nshortDescription: The %s plugin\n", p.name, p.version, p.name)
	fmt.Fprintf(&b, "requires:\n  host: %q\n  protocol: %q\n", p.host, p.protocol)
	fmt.Fprintf(&b, "feed:\n  organization: acme\n  feed: plugins\n  project: %q\n", p.project)
	if !p.noPlatform {
		fmt.Fprintf(&b, "platforms:\n  - selector: {os: %s, arch: %s}\n    package: %s-%s-%s\n",
			runtime.GOOS, runtime.GOARCH, p.name, runtime.GOOS, runtime.GOARCH)
	}
	fmt.Fprintf(&b, "support:\n  documentation: https://docs.example/%s\n  channel: \"#%s\"\n", p.name, p.name)
	return b.String()
}

func pkgName(name string) string { return name + "-" + runtime.GOOS + "-" + runtime.GOARCH }

// cacheIndex writes an index cache (as a previous `dongle update` would
// have) that's fresh, so index.Prepare's default mode uses it without
// contacting the feed.
func cacheIndex(t *testing.T, version string, plugins ...plugin) {
	t.Helper()
	dir := filepath.Join(state.DataDir(), "index", "plugins")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, p := range plugins {
		if err := os.WriteFile(filepath.Join(dir, p.name+".yaml"), []byte(p.yaml()), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(state.DataDir(), "index.version"), []byte(version), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := index.MarkChecked(); err != nil {
		t.Fatal(err)
	}
}

// installed records plugins as installed in state, with a binary on disk.
func installed(t *testing.T, plugins ...state.Installed) {
	t.Helper()
	st, err := state.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range plugins {
		if p.Entrypoint == "" {
			p.Entrypoint = "dongle-" + p.Name
		}
		dir := state.PluginVersionDir(p.Name, p.ActiveVersion)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, p.Entrypoint), []byte("old"), 0o755); err != nil {
			t.Fatal(err)
		}
		st.Plugins[p.Name] = p
	}
	if err := st.Save(); err != nil {
		t.Fatal(err)
	}
}

func loadState(t *testing.T) *state.State {
	t.Helper()
	st, err := state.Load()
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func corruptState(t *testing.T) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(state.DataDir(), "state.json"), []byte("{bad"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustContain(t *testing.T, what, got string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(got, w) {
			t.Errorf("%s missing %q; got:\n%s", what, w, got)
		}
	}
}

func mustNotContain(t *testing.T, what, got string, nots ...string) {
	t.Helper()
	for _, n := range nots {
		if strings.Contains(got, n) {
			t.Errorf("%s unexpectedly contains %q; got:\n%s", what, n, got)
		}
	}
}

const host, proto = "1.0.0", "v1"

var deploy = plugin{name: "deploy", version: "v1.2.0", host: ">=0.1.0", protocol: "v1"}

func writeIndexFile(t *testing.T, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(state.DataDir(), "index", "plugins", name+".yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// quiet runs fn with output captured and discarded, returning its code.
func quiet(t *testing.T, fn func() int) int {
	t.Helper()
	var code int
	testutil.Capture(t, func() { code = fn() })
	return code
}
