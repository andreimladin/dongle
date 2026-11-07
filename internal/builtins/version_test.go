package builtins

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andreimladin/dongle/internal/host"
	"github.com/andreimladin/dongle/internal/index"
	"github.com/andreimladin/dongle/internal/state"
	"github.com/andreimladin/dongle/internal/testutil"
)

func TestVersion(t *testing.T) {
	t.Run("fresh install", func(t *testing.T) {
		newEnv(t)
		var code int
		stdout, _ := testutil.Capture(t, func() { code = Version("1.4.0") })
		if code != 0 {
			t.Fatalf("exit %d", code)
		}
		lines := fieldsByLine(stdout)
		want := []string{host.Name + " 1.4.0", "index none (run `" + host.Name + " update`)", "", "no plugins installed"}
		if strings.Join(lines, "\n") != strings.Join(want, "\n") {
			t.Fatalf("got:\n%s", stdout)
		}
	})
	t.Run("embedded index and plugins", func(t *testing.T) {
		newEnv(t)
		cacheIndex(t, "2024.03.01")
		os.WriteFile(filepath.Join(state.DataDir(), "index.origin"), []byte(index.OriginEmbedded), 0o644)
		installed(t,
			state.Installed{Name: "zeta", ActiveVersion: "0.1.0"},
			state.Installed{Name: "alpha", ActiveVersion: "1.0.0"})
		stdout, _ := testutil.Capture(t, func() { Version("dev") })
		want := []string{host.Name + " dev", "index 2024.03.01 (embedded)", "", "installed plugins:", "alpha 1.0.0", "zeta 0.1.0"}
		if got := fieldsByLine(stdout); strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Fatalf("got:\n%s", stdout)
		}
	})
	t.Run("fetched index has no note", func(t *testing.T) {
		newEnv(t)
		cacheIndex(t, "2024.03.01")
		stdout, _ := testutil.Capture(t, func() { Version("dev") })
		if fieldsByLine(stdout)[1] != "index 2024.03.01" {
			t.Fatalf("got:\n%s", stdout)
		}
	})
	t.Run("unreadable state", func(t *testing.T) {
		newEnv(t)
		corruptState(t)
		var code int
		_, stderr := testutil.Capture(t, func() { code = Version("dev") })
		if code != 1 {
			t.Fatalf("exit %d", code)
		}
		mustContain(t, "stderr", stderr, "error: reading state:")
	})
}

// fieldsByLine normalizes column padding: each line's fields joined by a
// single space.
func fieldsByLine(s string) []string {
	var out []string
	for _, l := range strings.Split(strings.TrimRight(s, "\n"), "\n") {
		out = append(out, strings.Join(strings.Fields(l), " "))
	}
	return out
}
