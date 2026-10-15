package azcli

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fakeAz installs an `az` shell script as the only thing on PATH.
func fakeAz(t *testing.T, script string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("uses a shell-script az")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "az"), []byte("#!/bin/sh\n"+script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
}

func TestSystemLookPath(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if err := (System{}).LookPath(); err == nil {
		t.Fatal("want error with no az on PATH")
	}
	fakeAz(t, "exit 0\n")
	if err := (System{}).LookPath(); err != nil {
		t.Fatalf("az on PATH: %v", err)
	}
}

func TestSystemRunPassesArgsAndStreams(t *testing.T) {
	fakeAz(t, `echo "args: $*"; echo "oops" >&2; exit 0`+"\n")
	var stdout, stderr bytes.Buffer
	if err := (System{}).Run([]string{"artifacts", "universal", "download"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(stdout.String()); got != "args: artifacts universal download" {
		t.Fatalf("stdout = %q", got)
	}
	if got := strings.TrimSpace(stderr.String()); got != "oops" {
		t.Fatalf("stderr = %q", got)
	}
}

func TestSystemRunReportsFailure(t *testing.T) {
	fakeAz(t, "exit 3\n")
	if err := (System{}).Run(nil, nil, nil); err == nil {
		t.Fatal("want error for a nonzero az exit")
	}
}
