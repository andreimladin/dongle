package testutil

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestCaptureSeparatesStreams(t *testing.T) {
	origOut, origErr := os.Stdout, os.Stderr
	stdout, stderr := Capture(t, func() {
		fmt.Fprint(os.Stdout, "to out")
		fmt.Fprint(os.Stderr, "to err")
	})
	if stdout != "to out" || stderr != "to err" {
		t.Fatalf("got stdout %q, stderr %q", stdout, stderr)
	}
	if os.Stdout != origOut || os.Stderr != origErr {
		t.Fatal("Capture did not restore os.Stdout/os.Stderr")
	}
}

func TestCaptureLargeOutputDoesNotBlock(t *testing.T) {
	big := strings.Repeat("x", 1<<20) // well past a pipe buffer
	stdout, _ := Capture(t, func() { fmt.Fprint(os.Stdout, big) })
	if len(stdout) != len(big) {
		t.Fatalf("captured %d bytes, want %d", len(stdout), len(big))
	}
}

func TestIsolate(t *testing.T) {
	dir := Isolate(t)
	if got := os.Getenv("DONGLE_DATA_DIR"); got != dir {
		t.Fatalf("DONGLE_DATA_DIR = %q, want %q", got, dir)
	}
	if _, err := exec.LookPath("az"); err == nil {
		t.Fatal("az is still resolvable on PATH")
	}
}
