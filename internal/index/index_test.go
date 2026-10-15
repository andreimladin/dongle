package index

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/andreimladin/dongle/internal/state"
	"github.com/andreimladin/dongle/internal/testutil"
	"github.com/andreimladin/dongle/internal/ui"
)

// --- pure logic ----------------------------------------------------------------

func TestIsNewer(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"1.0.1", "1.0.0", true},
		{"1.0.0", "1.0.1", false},
		{"1.0.0", "1.0.0", false},
		{"1.10.0", "1.9.0", true}, // numeric, not lexical
		{"2024.03.01.2", "2024.03.01.1", true},
		{"2024.03.01", "2024.02.28.9", true},
		{"1.0.0.1", "1.0.0", true}, // more parts wins a tie
		{"1.0.0", "1.0.0.1", false},
		{"v1.2.0", "1.1.0", true}, // leading v ignored
		{" 1.2.0 ", "1.2.0", false},
		{"1.0.0", "", true}, // nothing recorded: anything is newer
		{"", "", false},
		{"", "1.0.0", false},
		{"1.0.b", "1.0.a", true}, // non-numeric part: string comparison
		{"1.0.a", "1.0.b", false},
	}
	for _, c := range cases {
		if got := IsNewer(c.a, c.b); got != c.want {
			t.Errorf("IsNewer(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestParseLatestVersion(t *testing.T) {
	body := []byte(`{"count":2,"value":[
		{"name":"dongle-index-extra","versions":[{"version":"9.9.9","isLatest":true}]},
		{"name":"Dongle-Index","versions":[{"version":"1.4.0","isLatest":true}]}]}`)
	got, err := parseLatestVersion(body, "dongle-index")
	if err != nil || got != "1.4.0" {
		t.Fatalf("got %q, %v; want 1.4.0", got, err)
	}

	noFlag := []byte(`{"value":[{"name":"dongle-index","versions":[{"version":"1.2.0"},{"version":"1.10.0"}]}]}`)
	if got, err := parseLatestVersion(noFlag, "dongle-index"); err != nil || got != "1.10.0" {
		t.Fatalf("got %q, %v; want 1.10.0", got, err)
	}

	if _, err := parseLatestVersion([]byte(`{"value":[]}`), "dongle-index"); err == nil {
		t.Fatal("want an error for a package with no versions")
	}
	if _, err := parseLatestVersion([]byte(`{"value":[{"name":"dongle-index","versions":[]}]}`), "dongle-index"); err == nil {
		t.Fatal("want an error for a matching package with an empty version list")
	}
	if _, err := parseLatestVersion([]byte(`<html>`), "dongle-index"); err == nil || !strings.Contains(err.Error(), "parse feed response") {
		t.Fatalf("want a parse error for a non-JSON body, got %v", err)
	}
}

func TestAzError(t *testing.T) {
	base := errors.New("exit status 1")
	err := AzError("az download x@1", base, []byte("  ERROR: unauthorized\n"))
	if !errors.Is(err, base) || err.Error() != "az download x@1: exit status 1\nERROR: unauthorized" {
		t.Fatalf("with stderr: %q", err)
	}
	if err := AzError("az download x@1", base, []byte(" \n")); err.Error() != "az download x@1: exit status 1" {
		t.Fatalf("blank stderr: %q", err)
	}
}

func TestFeedIdentityEnvOverrides(t *testing.T) {
	SetDefaults("o", "p", "f", "pkg")
	for _, k := range []string{"DONGLE_INDEX_ORG", "DONGLE_INDEX_PROJECT", "DONGLE_INDEX_FEED", "DONGLE_INDEX_PACKAGE"} {
		t.Setenv(k, "")
	}
	if got := []string{indexOrg(), indexProject(), indexFeed(), indexPackage()}; !reflect.DeepEqual(got, []string{"o", "p", "f", "pkg"}) {
		t.Fatalf("defaults: %v", got)
	}
	t.Setenv("DONGLE_INDEX_ORG", "O2")
	t.Setenv("DONGLE_INDEX_PROJECT", "P2")
	t.Setenv("DONGLE_INDEX_FEED", "F2")
	t.Setenv("DONGLE_INDEX_PACKAGE", "PKG2")
	if got := []string{indexOrg(), indexProject(), indexFeed(), indexPackage()}; !reflect.DeepEqual(got, []string{"O2", "P2", "F2", "PKG2"}) {
		t.Fatalf("overrides: %v", got)
	}
}

// --- archive extraction ----------------------------------------------------------

func TestExtractTarGz(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	tw.WriteHeader(&tar.Header{Name: "plugins/", Typeflag: tar.TypeDir, Mode: 0o755})
	tw.WriteHeader(&tar.Header{Name: "plugins/a.yaml", Typeflag: tar.TypeReg, Mode: 0o600, Size: 3})
	tw.Write([]byte("a:1"))
	tw.WriteHeader(&tar.Header{Name: "deep/nested/b", Typeflag: tar.TypeReg, Mode: 0o644, Size: 1})
	tw.Write([]byte("b"))
	tw.WriteHeader(&tar.Header{Name: "link", Typeflag: tar.TypeSymlink, Linkname: "/etc/passwd"})
	tw.Close()
	gz.Close()

	archive := filepath.Join(t.TempDir(), "index.tar.gz")
	if err := os.WriteFile(archive, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "out")
	if err := extractTarGz(archive, dest); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(filepath.Join(dest, "plugins", "a.yaml")); err != nil || string(b) != "a:1" {
		t.Fatalf("plugins/a.yaml = %q, %v", b, err)
	}
	if fi, err := os.Stat(filepath.Join(dest, "plugins", "a.yaml")); err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("file mode not preserved: %v, %v", fi.Mode(), err)
	}
	if b, err := os.ReadFile(filepath.Join(dest, "deep", "nested", "b")); err != nil || string(b) != "b" {
		t.Fatalf("parent dirs not created for deep/nested/b: %q, %v", b, err)
	}
	if _, err := os.Lstat(filepath.Join(dest, "link")); !os.IsNotExist(err) {
		t.Fatalf("symlink entries must be skipped, got %v", err)
	}
}

func TestExtractTarGzRejectsEscapes(t *testing.T) {
	for _, name := range []string{"../evil", "plugins/../../evil", "/../evil"} {
		archive := tarGz(t, map[string]string{name: "x"})
		dest := filepath.Join(t.TempDir(), "out")
		err := extractTarGzBytes(archive, dest)
		if err == nil || !strings.Contains(err.Error(), "escapes destination") {
			t.Errorf("entry %q: want an escape error, got %v", name, err)
		}
		if _, serr := os.Stat(filepath.Join(filepath.Dir(dest), "evil")); serr == nil {
			t.Errorf("entry %q was written outside the destination", name)
		}
	}
}

func TestExtractTarGzErrors(t *testing.T) {
	if err := extractTarGzBytes([]byte("not gzip"), filepath.Join(t.TempDir(), "out")); err == nil {
		t.Error("want error for non-gzip input")
	}
	if err := extractTarGz(filepath.Join(t.TempDir(), "missing.tar.gz"), t.TempDir()); err == nil {
		t.Error("want error for a missing archive")
	}
	// Valid gzip, truncated tar stream.
	full := tarGz(t, map[string]string{"a": strings.Repeat("x", 4096)})
	var trunc bytes.Buffer
	gz := gzip.NewWriter(&trunc)
	r, _ := gzip.NewReader(bytes.NewReader(full))
	raw := new(bytes.Buffer)
	raw.ReadFrom(r)
	gz.Write(raw.Bytes()[:1000])
	gz.Close()
	if err := extractTarGzBytes(trunc.Bytes(), filepath.Join(t.TempDir(), "out")); err == nil {
		t.Error("want error for a truncated tar stream")
	}
}

// --- manifests ------------------------------------------------------------------

func TestLoadAndList(t *testing.T) {
	newEnv(t)
	cache(t, "1.0.0", map[string]string{
		"deploy": `name: deploy
version: v1.2.0
shortDescription: Deploy things
requires:
  host: ">=0.1.0"
  protocol: v1
feed:
  organization: org
  project: proj
  feed: plugins
  packageType: upack
  packageName: deploy
platforms:
  - selector: {os: linux, arch: amd64}
    package: deploy-linux-amd64
support:
  documentation: https://docs.example/deploy
  channel: "#deploy"
`,
		"broken": "name: [unterminated",
		"other":  manifest("other", "0.1.0"),
	}, false)
	// Non-manifests in plugins/ are ignored by List.
	os.WriteFile(filepath.Join(cacheDir(), "plugins", "README.md"), []byte("x"), 0o644)
	os.Mkdir(filepath.Join(cacheDir(), "plugins", "sub.yaml"), 0o755)

	m, err := Load("deploy")
	if err != nil {
		t.Fatal(err)
	}
	if m.Name != "deploy" || m.Version != "v1.2.0" || m.Requires.Host != ">=0.1.0" || m.Requires.Protocol != "v1" ||
		m.Feed.Project != "proj" || m.Feed.PackageName != "deploy" ||
		len(m.Platforms) != 1 || m.Platforms[0].Package != "deploy-linux-amd64" || m.Platforms[0].Selector.Arch != "amd64" ||
		m.Support.Channel != "#deploy" || m.Support.Contact != "" {
		t.Fatalf("parsed manifest wrong: %+v", m)
	}

	if _, err := Load("nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Load(missing) err = %v, want ErrNotFound", err)
	}
	if _, err := Load("broken"); err == nil || !strings.Contains(err.Error(), "invalid manifest for broken") {
		t.Fatalf("Load(broken) err = %v", err)
	}

	list, err := List()
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, m := range list {
		names = append(names, m.Name)
	}
	if strings.Join(names, ",") != "deploy,other" {
		t.Fatalf("List() = %v, want deploy and other only (broken and non-yaml skipped)", names)
	}
}

func TestListWithoutCache(t *testing.T) {
	newEnv(t)
	if _, err := List(); err == nil {
		t.Fatal("want error listing with no cache")
	}
}

func TestLoadFileAndStrict(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.yaml")
	typo := filepath.Join(dir, "typo.yaml")
	bad := filepath.Join(dir, "bad.yaml")
	os.WriteFile(good, []byte(manifest("good", "1.0.0")), 0o644)
	os.WriteFile(typo, []byte(manifest("typo", "1.0.0")+"shortDescriptoin: oops\n"), 0o644)
	os.WriteFile(bad, []byte("version: [\n"), 0o644)

	for name, load := range map[string]func(string) (*Manifest, error){"LoadFile": LoadFile, "LoadFileStrict": LoadFileStrict} {
		if m, err := load(good); err != nil || m.Name != "good" {
			t.Errorf("%s(good) = %+v, %v", name, m, err)
		}
		if _, err := load(filepath.Join(dir, "missing.yaml")); !errors.Is(err, ErrNotFound) {
			t.Errorf("%s(missing) err = %v, want ErrNotFound", name, err)
		}
		if _, err := load(bad); err == nil || !strings.Contains(err.Error(), "invalid manifest at") {
			t.Errorf("%s(bad) err = %v", name, err)
		}
		if _, err := load(dir); err == nil || errors.Is(err, ErrNotFound) {
			t.Errorf("%s(dir) err = %v, want a read error", name, err)
		}
	}
	// Only the strict loader rejects unknown fields.
	if _, err := LoadFile(typo); err != nil {
		t.Errorf("LoadFile(typo) should ignore unknown fields, got %v", err)
	}
	if _, err := LoadFileStrict(typo); err == nil || !strings.Contains(err.Error(), "shortDescriptoin") {
		t.Errorf("LoadFileStrict(typo) err = %v, want the unknown field named", err)
	}
}

func TestPlatformFor(t *testing.T) {
	here := Selector{OS: runtime.GOOS, Arch: runtime.GOARCH}
	cases := []struct {
		name      string
		platforms []Platform
		want      string // package; "" means an error is expected
	}{
		{"exact match", []Platform{{Selector: here, Package: "p-here"}}, "p-here"},
		{"first match wins", []Platform{
			{Selector: Selector{OS: "plan9", Arch: runtime.GOARCH}, Package: "wrong-os"},
			{Selector: Selector{OS: runtime.GOOS, Arch: "mips"}, Package: "wrong-arch"},
			{Selector: here, Package: "p1"},
			{Selector: here, Package: "p2"},
		}, "p1"},
		{"no match", []Platform{{Selector: Selector{OS: "plan9", Arch: "mips"}, Package: "x"}}, ""},
		{"no platforms", nil, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := &Manifest{Name: "deploy", Version: "1.0.0", Platforms: c.platforms}
			p, err := m.PlatformFor()
			if c.want == "" {
				if err == nil || !strings.Contains(err.Error(), "deploy 1.0.0 has no build for "+runtime.GOOS+"/"+runtime.GOARCH) {
					t.Fatalf("err = %v", err)
				}
				return
			}
			if err != nil || p.Package != c.want {
				t.Fatalf("got %+v, %v; want %s", p, err, c.want)
			}
		})
	}
}

// --- cache metadata -------------------------------------------------------------

func TestIsFresh(t *testing.T) {
	t.Setenv("DONGLE_DATA_DIR", t.TempDir())

	if IsFresh(CheckTTL) {
		t.Fatal("never-checked cache reads as fresh")
	}
	if err := MarkChecked(); err != nil {
		t.Fatal(err)
	}
	if !IsFresh(CheckTTL) {
		t.Fatal("just-checked cache reads as stale")
	}
	old := time.Now().Add(-CheckTTL - time.Minute)
	if err := os.Chtimes(metaPath(), old, old); err != nil {
		t.Fatal(err)
	}
	if IsFresh(CheckTTL) {
		t.Fatal("cache checked over an hour ago reads as fresh")
	}
	future := time.Now().Add(time.Hour)
	if err := os.Chtimes(metaPath(), future, future); err != nil {
		t.Fatal(err)
	}
	if IsFresh(CheckTTL) {
		t.Fatal("a stamp from the future (clock skew) must not read as fresh")
	}
}

func TestCachedVersionAndOrigin(t *testing.T) {
	newEnv(t)
	if _, ok := CachedVersion(); ok {
		t.Fatal("no version file: want ok=false")
	}
	if _, ok := CachedOrigin(); ok {
		t.Fatal("nothing cached: want origin ok=false")
	}
	os.WriteFile(versionPath(), []byte(" \n"), 0o644)
	if _, ok := CachedVersion(); ok {
		t.Fatal("blank version file: want ok=false")
	}
	os.WriteFile(versionPath(), []byte("2024.1.2\n"), 0o644)
	if v, ok := CachedVersion(); !ok || v != "2024.1.2" {
		t.Fatalf("CachedVersion = %q, %v", v, ok)
	}
	// Caches from before origin tracking have no origin file: fetched.
	if o, ok := CachedOrigin(); !ok || o != OriginFetched {
		t.Fatalf("no origin file: %q, %v", o, ok)
	}
	writeOrigin(OriginEmbedded)
	if o, _ := CachedOrigin(); o != OriginEmbedded {
		t.Fatalf("embedded origin: %q", o)
	}
	writeOrigin("garbage")
	if o, _ := CachedOrigin(); o != OriginFetched {
		t.Fatalf("unknown origin should read as fetched, got %q", o)
	}
}

// --- feed access ----------------------------------------------------------------

func TestLatestVersionQuery(t *testing.T) {
	e := newEnv(t)
	e.feed.latest = "2024.5.1"

	SetDefaults("myorg", "", "myfeed", "dongle-index")
	if v, err := LatestVersion(); err != nil || v != "2024.5.1" {
		t.Fatalf("got %q, %v", v, err)
	}
	args := strings.Join(e.feed.calls[0], " ")
	mustContain(t, "org-scoped query", args,
		"devops invoke", "--organization https://dev.azure.com/myorg", "--area Packaging",
		"--route-parameters feedId=myfeed --query-parameters", "packageNameQuery=dongle-index")
	mustNotContain(t, "org-scoped query", args, "project=")

	SetDefaults("myorg", "myproj", "myfeed", "dongle-index")
	LatestVersion()
	mustContain(t, "project-scoped query", strings.Join(e.feed.calls[1], " "), "feedId=myfeed project=myproj --query-parameters")
}

func TestLatestVersionErrors(t *testing.T) {
	e := newEnv(t)
	e.feed.missing = true
	if _, err := LatestVersion(); !errors.Is(err, errAzMissing) {
		t.Fatalf("az missing: err = %v", err)
	}
	if len(e.feed.calls) != 0 {
		t.Fatal("az must not be run when it isn't installed")
	}

	e.feed.missing = false
	e.feed.queryErr = errors.New("exit status 1")
	_, err := LatestVersion()
	if err == nil {
		t.Fatal("want error")
	}
	mustContain(t, "query error", err.Error(), "az query latest dongle-index version", "exit status 1", "ERROR: unauthorized")
}

func TestFetch(t *testing.T) {
	t.Run("uses the archive's VERSION and stages under the data dir", func(t *testing.T) {
		e := newEnv(t)
		e.publish(t, "2.0.0", map[string]string{"deploy": manifest("deploy", "1.0.0")})
		l, err := Fetch("2.0.0")
		if err != nil {
			t.Fatal(err)
		}
		defer l.Discard()
		if l.Version != "2.0.0" {
			t.Fatalf("Version = %q", l.Version)
		}
		if !strings.HasPrefix(l.staging, filepath.Join(state.DataDir(), ".staging")) {
			t.Fatalf("staging %q is not under the data dir (Apply's rename must stay on one filesystem)", l.staging)
		}
		mustContain(t, "download args", strings.Join(e.feed.calls[0], " "),
			"artifacts universal download", "--organization https://dev.azure.com/org", "--feed feed", "--name dongle-index", "--version 2.0.0")
		if HasCache() {
			t.Fatal("Fetch must not touch the cache before Apply")
		}
	})
	t.Run("falls back to the requested version without a VERSION file", func(t *testing.T) {
		e := newEnv(t)
		e.feed.archives["3.0.0"] = indexArchive(t, "", nil)
		l, err := Fetch("3.0.0")
		if err != nil {
			t.Fatal(err)
		}
		defer l.Discard()
		if l.Version != "3.0.0" {
			t.Fatalf("Version = %q", l.Version)
		}
	})
	t.Run("latest without a VERSION file is unversioned", func(t *testing.T) {
		e := newEnv(t)
		e.feed.archives["*"] = indexArchive(t, "", nil)
		l, err := Fetch("*")
		if err != nil {
			t.Fatal(err)
		}
		defer l.Discard()
		if l.Version != "" {
			t.Fatalf("Version = %q, want empty", l.Version)
		}
	})
	t.Run("project-scoped feed", func(t *testing.T) {
		e := newEnv(t)
		SetDefaults("org", "proj", "feed", "dongle-index")
		e.publish(t, "1.0.0", nil)
		l, err := Fetch("1.0.0")
		if err != nil {
			t.Fatal(err)
		}
		l.Discard()
		mustContain(t, "download args", strings.Join(e.feed.calls[0], " "), "--project proj --scope project")
	})
}

func TestFetchErrorsCleanUpStaging(t *testing.T) {
	cases := []struct {
		name    string
		setup   func(t *testing.T, e *env)
		wantErr string
	}{
		{"az missing", func(t *testing.T, e *env) { e.feed.missing = true }, "the Azure CLI is required"},
		{"download fails", func(t *testing.T, e *env) { e.feed.downloadErr = errors.New("exit status 1") }, "ERROR: package not found"},
		{"package has extra files", func(t *testing.T, e *env) { e.publish(t, "1.0.0", nil); e.feed.extraFile = true }, "contains 2 files, expected exactly 1"},
		{"corrupt archive", func(t *testing.T, e *env) { e.feed.archives["1.0.0"] = []byte("garbage") }, "extract index archive"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t)
			c.setup(t, e)
			_, err := Fetch("1.0.0")
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Fatalf("err = %v, want %q", err, c.wantErr)
			}
			entries, _ := os.ReadDir(filepath.Join(state.DataDir(), ".staging"))
			if len(entries) != 0 {
				t.Fatalf("staging left behind: %v", entries)
			}
		})
	}
}

func TestApplyReplacesCache(t *testing.T) {
	e := newEnv(t)
	cache(t, "1.0.0", map[string]string{"old": manifest("old", "1.0.0")}, false)
	writeOrigin(OriginEmbedded)
	e.publish(t, "2.0.0", map[string]string{"new": manifest("new", "1.0.0")})

	l, err := Fetch("2.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Apply(); err != nil {
		t.Fatal(err)
	}
	if _, err := Load("old"); !errors.Is(err, ErrNotFound) {
		t.Fatal("old cache contents survived Apply")
	}
	if _, err := Load("new"); err != nil {
		t.Fatalf("new manifest not installed: %v", err)
	}
	if v := cachedVersion(t); v != "2.0.0" {
		t.Fatalf("version = %q", v)
	}
	if o, _ := CachedOrigin(); o != OriginFetched {
		t.Fatalf("origin = %q", o)
	}
	if !IsFresh(CheckTTL) {
		t.Fatal("a just-applied index must be fresh")
	}
	if _, err := os.Stat(l.staging); !os.IsNotExist(err) {
		t.Fatal("Apply left its staging dir behind")
	}
}

// --- embedded seed ----------------------------------------------------------------

func TestEnsureCache(t *testing.T) {
	t.Run("existing cache is left alone", func(t *testing.T) {
		e := newEnv(t)
		cache(t, "1.0.0", nil, false)
		e.embed(t, "9.9.9", nil)
		if err := EnsureCache(); err != nil {
			t.Fatal(err)
		}
		if v := cachedVersion(t); v != "1.0.0" || len(e.feed.calls) != 0 {
			t.Fatalf("version %q, az calls %v", v, e.feed.calls)
		}
	})
	t.Run("seeds from the embedded index offline", func(t *testing.T) {
		e := newEnv(t)
		e.embed(t, "0.5.0", map[string]string{"deploy": manifest("deploy", "1.0.0")})
		MarkChecked() // a stale stamp from some earlier cache
		if err := EnsureCache(); err != nil {
			t.Fatal(err)
		}
		if len(e.feed.calls) != 0 {
			t.Fatalf("seeding must not touch the feed: %v", e.feed.calls)
		}
		if v := cachedVersion(t); v != "0.5.0" {
			t.Fatalf("version = %q", v)
		}
		if o, _ := CachedOrigin(); o != OriginEmbedded {
			t.Fatalf("origin = %q", o)
		}
		if IsFresh(CheckTTL) {
			t.Fatal("a seeded cache was never confirmed against the feed; it must read as stale")
		}
		if _, err := Load("deploy"); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("downloads the latest when nothing is embedded", func(t *testing.T) {
		e := newEnv(t)
		e.publish(t, "3.0.0", nil)
		if err := EnsureCache(); err != nil {
			t.Fatal(err)
		}
		if got := e.feed.downloads(); !reflect.DeepEqual(got, []string{"*"}) {
			t.Fatalf("downloads = %v, want [*]", got)
		}
		if v := cachedVersion(t); v != "3.0.0" {
			t.Fatalf("version = %q", v)
		}
	})
	t.Run("corrupt embedded seed falls back to the feed", func(t *testing.T) {
		e := newEnv(t)
		embeddedIndex = func() ([]byte, string, bool) { return []byte("garbage"), "0.1.0", true }
		e.publish(t, "3.0.0", nil)
		var err error
		_, stderr := testutil.Capture(t, func() { err = EnsureCache() })
		if err != nil {
			t.Fatal(err)
		}
		mustContain(t, "stderr", stderr, "could not seed the embedded plugin index")
		if v := cachedVersion(t); v != "3.0.0" {
			t.Fatalf("version = %q", v)
		}
	})
}

func TestSeedEmbedded(t *testing.T) {
	e := newEnv(t)
	if ok, err := SeedEmbedded(); ok || err != nil {
		t.Fatalf("nothing embedded: ok=%v err=%v", ok, err)
	}
	if HasEmbedded() {
		t.Fatal("HasEmbedded with nothing embedded")
	}
	cache(t, "5.0.0", map[string]string{"stale": manifest("stale", "1.0.0")}, true)
	e.embed(t, "0.1.0", map[string]string{"seed": manifest("seed", "1.0.0")})
	if !HasEmbedded() {
		t.Fatal("HasEmbedded false with an embedded index")
	}
	if ok, err := SeedEmbedded(); !ok || err != nil {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if _, err := Load("stale"); !errors.Is(err, ErrNotFound) {
		t.Fatal("SeedEmbedded must replace the existing cache")
	}
	if v := cachedVersion(t); v != "0.1.0" || IsFresh(CheckTTL) {
		t.Fatalf("version %q fresh %v", v, IsFresh(CheckTTL))
	}
}

// --- Prepare (search/install/upgrade) ---------------------------------------------

func TestPrepare(t *testing.T) {
	type outcome struct {
		err       bool
		queried   bool     // asked the feed for its latest version
		downloads []string // versions downloaded
		asked     bool     // prompted the user
		version   string   // cached index version afterwards
		fresh     bool     // cache stamped as checked afterwards
		stderr    []string // substrings
		notStderr []string
	}
	const old, newer = "2024.1.1", "2024.2.1"
	cases := []struct {
		name     string
		mode     SyncMode
		mutating bool
		setup    func(t *testing.T, e *env)
		want     outcome
	}{
		{
			name: "first run downloads the latest without checking", mode: SyncAsk, mutating: true,
			setup: func(t *testing.T, e *env) { e.publish(t, newer, nil) },
			want:  outcome{downloads: []string{"*"}, version: newer, fresh: true, stderr: []string{"Downloaded plugin index " + newer + "."}},
		},
		{
			name: "first run with the feed unreachable fails", mode: SyncAsk, mutating: true,
			setup: func(t *testing.T, e *env) { e.feed.downloadErr = errors.New("exit status 1") },
			want:  outcome{err: true, downloads: []string{"*"}},
		},
		{
			name: "--no-sync never checks", mode: SyncNever, mutating: true,
			setup: func(t *testing.T, e *env) { cache(t, old, nil, false); e.publish(t, newer, nil) },
			want:  outcome{version: old, stderr: []string{"Using cached plugin index " + old + " (--no-sync)."}},
		},
		{
			name: "within the TTL nothing is checked or printed", mode: SyncAsk, mutating: true,
			setup: func(t *testing.T, e *env) { cache(t, old, nil, true); e.publish(t, newer, nil) },
			want:  outcome{version: old, fresh: true},
		},
		{
			name: "stale and current marks it checked", mode: SyncAsk, mutating: true,
			setup: func(t *testing.T, e *env) { cache(t, old, nil, false); e.feed.latest = old },
			want:  outcome{queried: true, version: old, fresh: true, stderr: []string{"Plugin index " + old + " is already up to date."}},
		},
		{
			name: "--sync checks even within the TTL", mode: SyncAlways, mutating: false,
			setup: func(t *testing.T, e *env) { cache(t, old, nil, true); e.publish(t, newer, nil) },
			want: outcome{queried: true, downloads: []string{newer}, version: newer, fresh: true,
				stderr: []string{"Updated plugin index " + old + " -> " + newer + "."}, notStderr: []string{"plugins:"}},
		},
		{
			name: "feed unreachable keeps the cache", mode: SyncAsk, mutating: true,
			setup: func(t *testing.T, e *env) { cache(t, old, nil, false); e.feed.queryErr = errors.New("exit status 1") },
			want: outcome{queried: true, version: old,
				stderr: []string{"warning: could not check the feed for a newer plugin index", "Using cached plugin index " + old + "."}},
		},
		{
			name: "search only notes a newer index", mode: SyncAsk, mutating: false,
			setup: func(t *testing.T, e *env) {
				cache(t, old, nil, false)
				e.publish(t, newer, nil)
				e.prompt.terminal = true
			},
			want: outcome{queried: true, version: old,
				stderr: []string{"note: a newer plugin index is available (" + old + " -> " + newer + ")", "pass --sync"}},
		},
		{
			name: "install without a terminal only notes it", mode: SyncAsk, mutating: true,
			setup: func(t *testing.T, e *env) { cache(t, old, nil, false); e.publish(t, newer, nil) },
			want:  outcome{queried: true, version: old, stderr: []string{"note: a newer plugin index is available"}},
		},
		{
			name: "install on a terminal, user accepts", mode: SyncAsk, mutating: true,
			setup: func(t *testing.T, e *env) {
				cache(t, old, nil, false)
				e.publish(t, newer, map[string]string{"deploy": manifest("deploy", "1.0.0")})
				e.prompt.terminal, e.prompt.answer = true, true
			},
			want: outcome{queried: true, asked: true, downloads: []string{newer}, version: newer, fresh: true,
				// mutating commands also show the new index's summary
				stderr: []string{"Updated plugin index " + old + " -> " + newer + ".", "plugins:", "deploy"}},
		},
		{
			name: "install on a terminal, user declines", mode: SyncAsk, mutating: true,
			setup: func(t *testing.T, e *env) {
				cache(t, old, nil, false)
				e.publish(t, newer, nil)
				e.prompt.terminal = true
			},
			want: outcome{queried: true, asked: true, version: old, stderr: []string{"Using cached plugin index " + old + "."}},
		},
		{
			name: "accepted update that fails to download keeps the cache", mode: SyncAlways, mutating: true,
			setup: func(t *testing.T, e *env) {
				cache(t, old, nil, false)
				e.feed.latest = newer
				e.feed.downloadErr = errors.New("exit status 1")
			},
			want: outcome{queried: true, downloads: []string{newer}, version: old,
				stderr: []string{"warning: could not update the plugin index", "Using cached plugin index " + old + "."}},
		},
		{
			name: "embedded seed is checked against the feed straight away", mode: SyncAsk, mutating: false,
			setup: func(t *testing.T, e *env) { e.embed(t, old, nil); e.feed.latest = old },
			want:  outcome{queried: true, version: old, fresh: true, stderr: []string{"already up to date"}},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t)
			c.setup(t, e)
			var err error
			stdout, stderr := testutil.Capture(t, func() { err = Prepare(c.mode, c.mutating) })
			w := c.want
			if (err != nil) != w.err {
				t.Fatalf("err = %v, want error %v", err, w.err)
			}
			if stdout != "" {
				t.Errorf("Prepare wrote to stdout: %q", stdout)
			}
			if got := e.feed.queries() > 0; got != w.queried {
				t.Errorf("queried feed = %v, want %v", got, w.queried)
			}
			if got := e.feed.downloads(); !reflect.DeepEqual(got, w.downloads) {
				t.Errorf("downloads = %v, want %v", got, w.downloads)
			}
			if got := len(e.prompt.asked) > 0; got != w.asked {
				t.Errorf("prompted = %v, want %v", got, w.asked)
			}
			if w.asked {
				mustContain(t, "prompt", e.prompt.asked[0], "A newer plugin index is available ("+old+" -> "+newer+")")
			}
			if got := cachedVersion(t); got != w.version {
				t.Errorf("cached version = %q, want %q", got, w.version)
			}
			if got := IsFresh(CheckTTL); got != w.fresh {
				t.Errorf("fresh = %v, want %v", got, w.fresh)
			}
			mustContain(t, "stderr", stderr, w.stderr...)
			mustNotContain(t, "stderr", stderr, w.notStderr...)
			if len(w.stderr) == 0 && !w.err && stderr != "" {
				t.Errorf("want silence, got stderr:\n%s", stderr)
			}
		})
	}
}

// --- Update (`dongle update`) --------------------------------------------------------

func TestUpdate(t *testing.T) {
	t.Run("current index is confirmed, not re-downloaded", func(t *testing.T) {
		e := newEnv(t)
		cache(t, "1.0.0", map[string]string{"deploy": manifest("deploy", "1.0.0")}, false)
		e.feed.latest = "1.0.0"
		var err error
		stdout, stderr := testutil.Capture(t, func() { err = Update() })
		if err != nil {
			t.Fatal(err)
		}
		if len(e.feed.downloads()) != 0 {
			t.Fatal("downloaded an index that was already current")
		}
		if !IsFresh(CheckTTL) {
			t.Fatal("confirmed index not stamped fresh")
		}
		mustContain(t, "stderr", stderr, "Plugin index 1.0.0 is already up to date.")
		mustContain(t, "stdout", stdout, "index", "1.0.0", "(already the latest)", "deploy")
	})
	t.Run("ignores the TTL and downloads a newer index", func(t *testing.T) {
		e := newEnv(t)
		cache(t, "1.0.0", nil, true)
		e.publish(t, "1.1.0", map[string]string{"deploy": manifest("deploy", "1.0.0")})
		var err error
		stdout, stderr := testutil.Capture(t, func() { err = Update() })
		if err != nil {
			t.Fatal(err)
		}
		if got := e.feed.downloads(); !reflect.DeepEqual(got, []string{"1.1.0"}) {
			t.Fatalf("downloads = %v", got)
		}
		mustContain(t, "stderr", stderr, "Updated plugin index 1.0.0 -> 1.1.0.")
		mustContain(t, "stdout", stdout, "1.1.0", "(updated from 1.0.0)", "deploy")
	})
	t.Run("first download", func(t *testing.T) {
		e := newEnv(t)
		e.publish(t, "1.1.0", nil)
		var err error
		stdout, stderr := testutil.Capture(t, func() { err = Update() })
		if err != nil {
			t.Fatal(err)
		}
		mustContain(t, "stderr", stderr, "Plugin index 1.1.0 is the latest.")
		mustContain(t, "stdout", stdout, "plugins:  none")
	})
	t.Run("feed failure keeps the cache and reports it", func(t *testing.T) {
		e := newEnv(t)
		cache(t, "1.0.0", nil, false)
		e.feed.queryErr = errors.New("exit status 1")
		var err error
		stdout, stderr := testutil.Capture(t, func() { err = Update() })
		if err == nil {
			t.Fatal("want error")
		}
		if stdout != "" {
			t.Fatalf("stdout = %q", stdout)
		}
		mustContain(t, "stderr", stderr, "error: could not update the plugin index", "Still using plugin index 1.0.0.")
	})
	t.Run("download failure with nothing cached falls back to the seed", func(t *testing.T) {
		e := newEnv(t)
		e.embed(t, "0.9.0", nil)
		e.feed.latest = "1.0.0"
		e.feed.downloadErr = errors.New("exit status 1")
		var err error
		_, stderr := testutil.Capture(t, func() { err = Update() })
		if err == nil {
			t.Fatal("want error")
		}
		if o, _ := CachedOrigin(); o != OriginEmbedded {
			t.Fatalf("origin = %q, want the embedded seed", o)
		}
		mustContain(t, "stderr", stderr, "Still using plugin index 0.9.0.")
	})
	t.Run("seed fallback failure is a warning", func(t *testing.T) {
		e := newEnv(t)
		embeddedIndex = func() ([]byte, string, bool) { return []byte("garbage"), "0.9.0", true }
		e.feed.queryErr = errors.New("exit status 1")
		var err error
		_, stderr := testutil.Capture(t, func() { err = Update() })
		if err == nil {
			t.Fatal("want error")
		}
		mustContain(t, "stderr", stderr, "warning: could not seed the embedded plugin index")
		mustNotContain(t, "stderr", stderr, "Still using")
	})
}

// --- WriteSummary ---------------------------------------------------------------

func TestWriteSummary(t *testing.T) {
	newEnv(t)
	cache(t, "2.0.0", map[string]string{
		"zeta":    manifest("zeta", "v1.0.0"),
		"alpha":   manifest("alpha", "2.0.0"),
		"current": manifest("current", "1.0.0"),
		"ahead":   manifest("ahead", "1.0.0"),
	}, false)
	st := &state.State{Plugins: map[string]state.Installed{
		"alpha":   {Name: "alpha", ActiveVersion: "1.0.0"},
		"current": {Name: "current", ActiveVersion: "1.0.0"},
		"ahead":   {Name: "ahead", ActiveVersion: "3.0.0"},
	}}
	if err := st.Save(); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	WriteSummary(&buf, ui.Palette{}, "1.0.0")
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 6 {
		t.Fatalf("got %d lines:\n%s", len(lines), buf.String())
	}
	mustContain(t, "index line", lines[0], "index", "2.0.0", "(updated from 1.0.0)")
	if lines[1] != "plugins:" {
		t.Fatalf("heading = %q", lines[1])
	}
	// Sorted by name, v prefix stripped, install state noted.
	checks := []struct{ name, version, note string }{
		{"ahead", "1.0.0", "installed"},
		{"alpha", "2.0.0", "installed 1.0.0, upgrade available"},
		{"current", "1.0.0", "installed"},
		{"zeta", "1.0.0", ""},
	}
	for i, c := range checks {
		f := strings.Fields(lines[i+2])
		if f[0] != c.name || f[1] != c.version || strings.Join(f[2:], " ") != c.note {
			t.Errorf("row %d = %q, want %s %s %s", i, lines[i+2], c.name, c.version, c.note)
		}
	}

	for prev, want := range map[string]string{"2.0.0": "index 2.0.0 (already the latest)", "": "index 2.0.0"} {
		buf.Reset()
		WriteSummary(&buf, ui.Palette{}, prev)
		first := strings.SplitN(buf.String(), "\n", 2)[0]
		if got := strings.Join(strings.Fields(first), " "); got != want {
			t.Errorf("prev %q: index line = %q, want %q", prev, got, want)
		}
	}
}

func TestWriteSummaryEmptyIndexAndBadState(t *testing.T) {
	newEnv(t)
	cache(t, "1.0.0", map[string]string{"deploy": manifest("deploy", "1.0.0")}, false)
	// Unreadable state must not break the summary: plugins just show as
	// not installed.
	os.WriteFile(filepath.Join(state.DataDir(), "state.json"), []byte("{bad"), 0o644)
	var buf bytes.Buffer
	WriteSummary(&buf, ui.Palette{}, "")
	mustContain(t, "summary", buf.String(), "deploy")
	mustNotContain(t, "summary", buf.String(), "installed")

	os.RemoveAll(filepath.Join(cacheDir(), "plugins"))
	buf.Reset()
	WriteSummary(&buf, ui.Palette{}, "")
	mustContain(t, "summary", buf.String(), "plugins:  none")
}
