package testutil

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"strings"
	"testing"
)

// MustContain fails the test (without stopping it) for each of wants that
// got doesn't contain. what names got in the failure message, e.g.
// "stderr". Asserting on stable substrings keeps tests from breaking on
// unrelated wording or spacing changes.
func MustContain(t testing.TB, what, got string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(got, w) {
			t.Errorf("%s missing %q; got:\n%s", what, w, got)
		}
	}
}

// MustNotContain is MustContain's opposite.
func MustNotContain(t testing.TB, what, got string, nots ...string) {
	t.Helper()
	for _, n := range nots {
		if strings.Contains(got, n) {
			t.Errorf("%s unexpectedly contains %q; got:\n%s", what, n, got)
		}
	}
}

// TarGz builds a tar+gzip archive in memory holding files (path ->
// contents), the format the plugin index is published in.
func TarGz(t testing.TB, files map[string]string) []byte {
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
