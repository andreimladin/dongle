package azclitest

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/andreimladin/dongle/internal/azcli"
)

func query(pkg string) []string {
	return []string{"devops", "invoke", "--area", "Packaging", "--query-parameters", "packageNameQuery=" + pkg}
}

func download(name, version, dest string) []string {
	return []string{"artifacts", "universal", "download", "--name", name, "--version", version, "--path", dest}
}

func TestInstallSwapsAndRestoresTheDefaultRunner(t *testing.T) {
	orig := azcli.Default
	t.Run("installed", func(t *testing.T) {
		f := New("idx")
		Install(t, f)
		if azcli.Default != azcli.Runner(f) {
			t.Fatal("Install did not make the fake the default runner")
		}
	})
	if azcli.Default != orig {
		t.Fatal("the real runner was not restored after the test")
	}
}

func TestQuery(t *testing.T) {
	f := New("idx")
	var stdout, stderr bytes.Buffer
	if err := f.Run(query("idx"), &stdout, &stderr); err == nil || !strings.Contains(stderr.String(), "idx not found") {
		t.Fatalf("nothing published: err %v, stderr %q", err, stderr.String())
	}

	f.Publish("1.2.0", []byte("archive"))
	stdout.Reset()
	if err := f.Run(query("idx"), &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if want := `{"value":[{"name":"idx","versions":[{"version":"1.2.0","isLatest":true}]}]}`; stdout.String() != want {
		t.Fatalf("query answer = %s, want %s", stdout.String(), want)
	}

	f.QueryErr = errors.New("exit status 1")
	stderr.Reset()
	if err := f.Run(query("idx"), &stdout, &stderr); err == nil || stderr.String() != "ERROR: unauthorized" {
		t.Fatalf("QueryErr: err %v, stderr %q", err, stderr.String())
	}
	if f.Queries() != 3 {
		t.Fatalf("Queries() = %d, want 3", f.Queries())
	}
}

func TestDownloads(t *testing.T) {
	f := New("idx")
	f.Publish("1.0.0", []byte("index-archive"))

	dir := t.TempDir()
	if err := f.Run(download("idx", "*", dir), nil, nil); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "index.tar.gz")); string(b) != "index-archive" {
		t.Fatalf("index download wrote %q", b)
	}

	dir = t.TempDir()
	if err := f.Run(download("deploy-linux-amd64", "1.2.0", dir), nil, nil); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "deploy-linux-amd64-pkg.bin")); !strings.Contains(string(b), "deploy-linux-amd64 1.2.0") {
		t.Fatalf("plugin download wrote %q", b)
	}

	if got := f.IndexDownloads(); !reflect.DeepEqual(got, []string{"*"}) {
		t.Errorf("IndexDownloads() = %v", got)
	}
	if got := f.PluginDownloads(); !reflect.DeepEqual(got, []string{"deploy-linux-amd64"}) {
		t.Errorf("PluginDownloads() = %v", got)
	}
	if len(f.PluginCalls()) != 1 || len(f.Calls) != 2 {
		t.Errorf("PluginCalls() = %v, Calls = %v", f.PluginCalls(), f.Calls)
	}
}

func TestDownloadFailures(t *testing.T) {
	cases := []struct {
		name       string
		setup      func(f *Feed)
		pkg        string
		wantStderr string
	}{
		{"index version not published", func(f *Feed) {}, "idx", "ERROR: package idx not found"},
		{"every download fails", func(f *Feed) { f.DownloadErr = errors.New("exit status 1") }, "deploy", "ERROR: package deploy not found"},
		{"one package fails", func(f *Feed) { f.FailPackages["deploy"] = true }, "deploy", "ERROR: unauthorized"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := New("idx")
			c.setup(f)
			var stderr bytes.Buffer
			dir := t.TempDir()
			if err := f.Run(download(c.pkg, "1.0.0", dir), nil, &stderr); err == nil {
				t.Fatal("want error")
			}
			if stderr.String() != c.wantStderr {
				t.Fatalf("stderr = %q, want %q", stderr.String(), c.wantStderr)
			}
			if entries, _ := os.ReadDir(dir); len(entries) != 0 {
				t.Fatalf("a failed download wrote %v", entries)
			}
		})
	}
}

func TestExtraFileAndMissing(t *testing.T) {
	f := New("idx")
	f.ExtraFile = true
	dir := t.TempDir()
	if err := f.Run(download("deploy", "1.0.0", dir), nil, nil); err != nil {
		t.Fatal(err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 2 {
		t.Fatalf("ExtraFile: got %d files, want 2", len(entries))
	}

	if err := f.LookPath(); err != nil {
		t.Fatalf("LookPath with az present: %v", err)
	}
	f.Missing = true
	if err := f.LookPath(); err == nil {
		t.Fatal("Missing: LookPath must fail")
	}
}

func TestUnknownCommandIsAnError(t *testing.T) {
	if err := New("idx").Run([]string{"account", "show"}, nil, nil); err == nil {
		t.Fatal("an az command the fake doesn't model must fail, not silently succeed")
	}
}

func TestFlagValue(t *testing.T) {
	args := []string{"--name", "deploy", "--version"}
	if got := FlagValue(args, "--name"); got != "deploy" {
		t.Errorf("FlagValue(--name) = %q", got)
	}
	if got := FlagValue(args, "--version"); got != "" {
		t.Errorf("FlagValue of a trailing flag with no value = %q, want empty", got)
	}
	if got := FlagValue(args, "--path"); got != "" {
		t.Errorf("FlagValue of an absent flag = %q, want empty", got)
	}
}
