package builtins

import (
	"testing"

	"github.com/andreimladin/dongle/internal/testutil"
)

func TestUpdateFailureIsExitOne(t *testing.T) {
	newEnv(t) // no az on PATH: the feed check fails
	var code int
	_, stderr := testutil.Capture(t, func() { code = Update() })
	if code != 1 {
		t.Fatalf("exit %d", code)
	}
	mustContain(t, "stderr", stderr, "could not update the plugin index")
}
