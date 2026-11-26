package index

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/andreimladin/dongle/internal/azcli/azclitest"
	"github.com/andreimladin/dongle/internal/testutil"
)

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
	feed   *azclitest.Feed
	prompt *fakePrompt
}

func newEnv(t *testing.T) *env {
	t.Helper()
	testutil.Isolate(t)
	e := &env{feed: azclitest.New("dongle-index"), prompt: &fakePrompt{}}
	azclitest.Install(t, e.feed)
	origPrompt := prompt
	prompt = e.prompt
	t.Cleanup(func() { prompt = origPrompt })
	SetDefaults("org", "", "feed", "dongle-index")
	return e
}

// publish registers an index archive with the given version and plugins on
// the fake feed, under both its version and "*".
func (e *env) publish(t *testing.T, version string, plugins map[string]string) {
	t.Helper()
	e.feed.Publish(version, indexArchive(t, version, plugins))
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
	return testutil.TarGz(t, files)
}

func cachedVersion(t *testing.T) string {
	t.Helper()
	v, _ := CachedVersion()
	return v
}
