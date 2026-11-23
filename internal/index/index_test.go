package index

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

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
}

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
}

func TestParseDefaults(t *testing.T) {
	got, err := parseDefaults([]byte("defaults:\n  - tacho\n  - bell\n  - ''\n  - tacho\n"))
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"tacho", "bell"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if got, err := parseDefaults(nil); err != nil || len(got) != 0 {
		t.Fatalf("empty file: got %v, %v", got, err)
	}
	if _, err := parseDefaults([]byte("default:\n  - tacho\n")); err == nil {
		t.Fatal("unknown key: want error")
	}
}

func TestParseDefaultsFileMissing(t *testing.T) {
	got, err := ParseDefaultsFile(filepath.Join(t.TempDir(), DefaultsFile))
	if err != nil || got != nil {
		t.Fatalf("missing file: got %v, %v", got, err)
	}
}
