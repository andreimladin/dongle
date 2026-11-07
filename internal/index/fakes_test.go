package index

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andreimladin/dongle/internal/testutil"
)

// fakeFeed stands in for the `az` CLI talking to the Azure Artifacts feed:
// `az devops invoke` answers with latest, `az artifacts universal
// download` drops the archive registered for the requested version.
type fakeFeed struct {
	missing     bool              // az not installed
	latest      string            // what the feed reports as the latest index version
	queryErr    error             // fail `devops invoke`
	archives    map[string][]byte // version ("*" for latest) -> index.tar.gz bytes
	downloadErr error             // fail `artifacts universal download`
	extraFile   bool              // download drops a second file next to the archive
	calls       [][]string
}

func (f *fakeFeed) LookPath() error {
	if f.missing {
		return errors.New("exec: \"az\": executable file not found in $PATH")
	}
	return nil
}

func (f *fakeFeed) Run(args []string, stdout, stderr io.Writer) error {
	f.calls = append(f.calls, args)
	switch {
	case len(args) > 1 && args[0] == "devops" && args[1] == "invoke":
		if f.queryErr != nil {
			fmt.Fprint(stderr, "ERROR: unauthorized")
			return f.queryErr
		}
		body, _ := json.Marshal(map[string]any{"value": []any{map[string]any{
			"name":     argAfter(args, "packageNameQuery="),
			"versions": []any{map[string]any{"version": f.latest, "isLatest": true}},
		}}})
		_, _ = stdout.Write(body)
		return nil
	case len(args) > 2 && args[0] == "artifacts" && args[2] == "download":
		if f.downloadErr != nil {
			fmt.Fprint(stderr, "ERROR: package not found")
			return f.downloadErr
		}
		dest, version := flagValue(args, "--path"), flagValue(args, "--version")
		archive, ok := f.archives[version]
		if !ok {
			return fmt.Errorf("fake feed has no archive for version %q", version)
		}
		if err := os.WriteFile(filepath.Join(dest, "index.tar.gz"), archive, 0o644); err != nil {
			return err
		}
		if f.extraFile {
			return os.WriteFile(filepath.Join(dest, "README"), nil, 0o644)
		}
		return nil
	}
	return fmt.Errorf("fake feed: unexpected az %v", args)
}

// downloads returns the --version of every download call, in order.
func (f *fakeFeed) downloads() []string {
	var out []string
	for _, c := range f.calls {
		if c[0] == "artifacts" {
			out = append(out, flagValue(c, "--version"))
		}
	}
	return out
}

func (f *fakeFeed) queries() int {
	n := 0
	for _, c := range f.calls {
		if c[0] == "devops" {
			n++
		}
	}
	return n
}

func flagValue(args []string, flag string) string {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == flag {
			return args[i+1]
		}
	}
	return ""
}

// argAfter returns the remainder of the first arg with the given prefix.
func argAfter(args []string, prefix string) string {
	for _, a := range args {
		if strings.HasPrefix(a, prefix) {
			return strings.TrimPrefix(a, prefix)
		}
	}
	return ""
}

// fakePrompt records whether it was asked and answers with answer.
type fakePrompt struct {
	terminal bool
	answer   bool
	asked    []string
}

func (p *fakePrompt) CanPrompt() bool { return p.terminal }
func (p *fakePrompt) Confirm(q string) bool {
	p.asked = append(p.asked, q)
	return p.answer
}

// env is one isolated test environment: an empty data dir and fakes for
// every external dependency of this package.
type env struct {
	feed   *fakeFeed
	prompt *fakePrompt
}

func newEnv(t *testing.T) *env {
	t.Helper()
	testutil.Isolate(t)
	e := &env{feed: &fakeFeed{archives: map[string][]byte{}}, prompt: &fakePrompt{}}
	origAz, origEmbedded, origPrompt := az, embeddedIndex, prompt
	az, prompt = e.feed, e.prompt
	embeddedIndex = func() ([]byte, string, bool) { return nil, "", false }
	t.Cleanup(func() { az, embeddedIndex, prompt = origAz, origEmbedded, origPrompt })
	SetDefaults("org", "", "feed", "dongle-index")
	return e
}

// publish registers an index archive with the given version and plugins on
// the fake feed, under both its version and "*".
func (e *env) publish(t *testing.T, version string, plugins map[string]string) {
	t.Helper()
	a := indexArchive(t, version, plugins)
	e.feed.archives[version] = a
	e.feed.archives["*"] = a
	e.feed.latest = version
}

func (e *env) embed(t *testing.T, version string, plugins map[string]string) {
	t.Helper()
	a := indexArchive(t, version, plugins)
	embeddedIndex = func() ([]byte, string, bool) { return a, version, true }
}

// cache writes an index straight into the cache, as a previous download
// would have, optionally stamping it as just checked.
func cache(t *testing.T, version string, plugins map[string]string, fresh bool) {
	t.Helper()
	dir := filepath.Join(cacheDir(), "plugins")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range plugins {
		if err := os.WriteFile(filepath.Join(dir, name+".yaml"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(versionPath(), []byte(version), 0o644); err != nil {
		t.Fatal(err)
	}
	if fresh {
		if err := MarkChecked(); err != nil {
			t.Fatal(err)
		}
	}
}

func manifest(name, version string) string {
	return fmt.Sprintf("name: %s\nversion: %s\nshortDescription: the %s plugin\n", name, version, name)
}

func indexArchive(t *testing.T, version string, plugins map[string]string) []byte {
	t.Helper()
	files := map[string]string{}
	if version != "" {
		files["VERSION"] = version + "\n"
	}
	for name, body := range plugins {
		files["plugins/"+name+".yaml"] = body
	}
	return tarGz(t, files)
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

func cachedVersion(t *testing.T) string {
	t.Helper()
	v, _ := CachedVersion()
	return v
}
