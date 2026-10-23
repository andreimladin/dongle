package builtins

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/andreimladin/dongle/internal/bootstrap"
	"github.com/andreimladin/dongle/internal/index"
	"github.com/andreimladin/dongle/internal/state"
	"github.com/andreimladin/dongle/internal/testutil"
)

func TestMain(m *testing.M) {
	// Entrypoints are named after the running host binary; pin it so
	// expectations don't depend on the test binary's name.
	os.Args[0] = "/usr/local/bin/dongle"
	os.Exit(m.Run())
}

// fakeAz stands in for `az artifacts universal download`: it drops a
// plugin binary (named unpredictably, as real packages may be) into --path.
type fakeAz struct {
	missing bool
	failFor map[string]bool // package names whose download fails
	extra   bool            // also drop a second file
	calls   [][]string
}

func (f *fakeAz) LookPath() error {
	if f.missing {
		return errors.New("not found")
	}
	return nil
}

func (f *fakeAz) Run(args []string, stdout, stderr io.Writer) error {
	f.calls = append(f.calls, args)
	name, version, dest := flagValue(args, "--name"), flagValue(args, "--version"), flagValue(args, "--path")
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

// newEnv isolates the test (temp data dir, empty PATH so internal/index
// can never reach a real az) and installs a fake az for artifact
// downloads.
func newEnv(t *testing.T) *fakeAz {
	t.Helper()
	testutil.Isolate(t)
	f := &fakeAz{failFor: map[string]bool{}}
	orig := az
	az = f
	t.Cleanup(func() { az = orig })
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

// fakePayload stands in for what a batteries-included binary embeds.
type fakePayload struct {
	hasIndex    bool
	seedErr     error
	defaults    []bootstrap.Default
	installErr  map[string]error
	markErr     error
	seeded      bool
	installedAs []string
	marked      bool
}

func (p *fakePayload) HasIndex() bool { return p.hasIndex }
func (p *fakePayload) SeedIndex() error {
	p.seeded = true
	return p.seedErr
}
func (p *fakePayload) PendingDefaults() []bootstrap.Default { return p.defaults }
func (p *fakePayload) InstallDefault(d bootstrap.Default) error {
	if err := p.installErr[d.Name]; err != nil {
		return err
	}
	p.installedAs = append(p.installedAs, d.Name)
	return nil
}
func (p *fakePayload) MarkDefaultsBootstrapped() error {
	p.marked = true
	return p.markErr
}
