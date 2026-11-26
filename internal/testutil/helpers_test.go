package testutil

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"testing"
)

// recorder is a testing.TB that records Errorf calls instead of failing.
type recorder struct {
	testing.TB
	errors []string
}

func (r *recorder) Helper() {}
func (r *recorder) Errorf(format string, a ...any) {
	r.errors = append(r.errors, fmt.Sprintf(format, a...))
}

func TestMustContain(t *testing.T) {
	r := &recorder{TB: t}
	MustContain(r, "stderr", "error: boom\n", "error:", "boom")
	MustNotContain(r, "stderr", "error: boom\n", "warning")
	if len(r.errors) != 0 {
		t.Fatalf("passing checks reported failures: %v", r.errors)
	}

	MustContain(r, "stderr", "error: boom\n", "warning", "boom", "note")
	MustNotContain(r, "stderr", "error: boom\n", "boom")
	if len(r.errors) != 3 {
		t.Fatalf("want one failure per missing/unwanted substring (3), got %d: %v", len(r.errors), r.errors)
	}
}

func TestTarGz(t *testing.T) {
	archive := TarGz(t, map[string]string{"VERSION": "1.0\n", "plugins/a.yaml": "name: a\n"})
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(tr)
		got[h.Name] = string(b)
	}
	if len(got) != 2 || got["VERSION"] != "1.0\n" || got["plugins/a.yaml"] != "name: a\n" {
		t.Fatalf("archive holds %v", got)
	}
}
