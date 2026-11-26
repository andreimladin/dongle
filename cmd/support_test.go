package main

import (
	"testing"

	"github.com/andreimladin/dongle/internal/testutil"
)

func TestSupport(t *testing.T) {
	testutil.Isolate(t)
	cacheIndex(t)
	code, stdout, stderr := run(t, "support", "deploy")
	if code != 0 {
		t.Fatalf("exit %d; stderr:\n%s", code, stderr)
	}
	mustContain(t, "stdout", stdout, "deploy — support", "https://docs.example/deploy", "#deploy")
}

func TestSupportErrors(t *testing.T) {
	testutil.Isolate(t)
	cacheIndex(t)
	code, _, stderr := run(t, "support", "nope")
	if code != 1 {
		t.Fatalf("unknown plugin: exit %d, want 1", code)
	}
	mustContain(t, "stderr", stderr, "no plugin named nope in the index")

	code, _, stderr = run(t, "support")
	if code != 2 {
		t.Fatalf("no plugin name: exit %d, want 2", code)
	}
	mustContain(t, "stderr", stderr, "accepts 1 arg(s), received 0")
}
