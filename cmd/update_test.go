package main

import (
	"testing"

	"github.com/andreimladin/dongle/internal/testutil"
)

func TestUpdate(t *testing.T) {
	testutil.Isolate(t) // no az on PATH: the feed can't be reached
	code, stdout, stderr := run(t, "update")
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want nothing on failure", stdout)
	}
	mustContain(t, "stderr", stderr, "error: could not update the plugin index")
}

func TestUpdateTakesNoArgs(t *testing.T) {
	testutil.Isolate(t)
	code, _, stderr := run(t, "update", "deploy")
	if code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
	mustContain(t, "stderr", stderr, `unknown command "deploy"`, "Run 'dongle update --help' for usage.")
}
