// Package testutil holds helpers shared by dongle's tests. It is imported
// only from _test.go files and never by production code.
package testutil

import (
	"bytes"
	"io"
	"os"
	"sync"
	"testing"
)

// Capture runs fn with os.Stdout and os.Stderr redirected to pipes and
// returns what it wrote to each. Pipes are not terminals, so output is
// plain (no color, no spinner animation), just as when piped in a script.
func Capture(t testing.TB, fn func()) (stdout, stderr string) {
	t.Helper()
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	origOut, origErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = outW, errW
	defer func() { os.Stdout, os.Stderr = origOut, origErr }()

	// Drain both pipes concurrently so a chatty fn can't fill a pipe
	// buffer and block.
	var wg sync.WaitGroup
	var outBuf, errBuf bytes.Buffer
	wg.Add(2)
	go func() { defer wg.Done(); _, _ = io.Copy(&outBuf, outR) }()
	go func() { defer wg.Done(); _, _ = io.Copy(&errBuf, errR) }()

	func() {
		defer outW.Close()
		defer errW.Close()
		fn()
	}()
	wg.Wait()
	outR.Close()
	errR.Close()
	return outBuf.String(), errBuf.String()
}

// Isolate points the data dir at a fresh temp dir and empties PATH for
// the duration of the test, so nothing can touch the real ~/.dongle or
// shell out to a real `az` (and through it the real feed).
func Isolate(t *testing.T) (dataDir string) {
	t.Helper()
	dataDir = t.TempDir()
	t.Setenv("DONGLE_DATA_DIR", dataDir)
	t.Setenv("PATH", t.TempDir())
	return dataDir
}
