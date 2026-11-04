package builtins

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andreimladin/dongle/internal/index"
	"github.com/andreimladin/dongle/internal/state"
	"github.com/andreimladin/dongle/internal/testutil"
)

func TestSearch(t *testing.T) {
	newEnv(t)
	cacheIndex(t, "1",
		plugin{name: "zeta", version: "v0.1.0"},
		plugin{name: "alpha", version: "2.0.0"})
	var code int
	stdout, _ := testutil.Capture(t, func() { code = Search(index.SyncAsk) })
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	want := []string{"alpha 2.0.0 The alpha plugin", "zeta 0.1.0 The zeta plugin"}
	if len(lines) != len(want) {
		t.Fatalf("stdout:\n%s", stdout)
	}
	for i := range want {
		if got := strings.Join(strings.Fields(lines[i]), " "); got != want[i] {
			t.Errorf("line %d = %q, want %q", i, got, want[i])
		}
	}
}

func TestSearchEdgeCases(t *testing.T) {
	t.Run("empty index", func(t *testing.T) {
		newEnv(t)
		cacheIndex(t, "1")
		var code int
		stdout, stderr := testutil.Capture(t, func() { code = Search(index.SyncAsk) })
		if code != 0 || stdout != "" {
			t.Fatalf("exit %d stdout %q", code, stdout)
		}
		mustContain(t, "stderr", stderr, "The plugin index is empty.")
	})
	t.Run("no index available", func(t *testing.T) {
		newEnv(t)
		if code := quiet(t, func() int { return Search(index.SyncAsk) }); code != 1 {
			t.Fatalf("exit %d", code)
		}
	})
	t.Run("cache without a plugins dir", func(t *testing.T) {
		newEnv(t)
		cacheIndex(t, "1")
		os.RemoveAll(filepath.Join(state.DataDir(), "index", "plugins"))
		if code := quiet(t, func() int { return Search(index.SyncNever) }); code != 1 {
			t.Fatalf("exit %d", code)
		}
	})
}

func TestSupport(t *testing.T) {
	newEnv(t)
	cacheIndex(t, "1", deploy)
	// Appending a contact to the rendered manifest keeps it valid YAML.
	writeIndexFile(t, "withcontact", strings.Replace(deploy.yaml(), "name: deploy", "name: withcontact", 1)+"  contact: ops@example.com\n")

	var code int
	stdout, _ := testutil.Capture(t, func() { code = Support("deploy") })
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	mustContain(t, "stdout", stdout, "deploy — support", "Documentation:", "https://docs.example/deploy", "Channel:", "#deploy")
	mustNotContain(t, "stdout", stdout, "Contact:")

	stdout, _ = testutil.Capture(t, func() { code = Support("withcontact") })
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	mustContain(t, "stdout", stdout, "Contact:", "ops@example.com")
}

func TestSupportFailures(t *testing.T) {
	newEnv(t)
	if code := quiet(t, func() int { return Support("deploy") }); code != 1 {
		t.Fatalf("no index: exit %d", code)
	}
	cacheIndex(t, "1")
	var code int
	_, stderr := testutil.Capture(t, func() { code = Support("deploy") })
	if code != 1 {
		t.Fatalf("exit %d", code)
	}
	mustContain(t, "stderr", stderr, "no plugin named deploy in the index")
	writeIndexFile(t, "deploy", "name: [")
	if code := quiet(t, func() int { return Support("deploy") }); code != 1 {
		t.Fatalf("invalid manifest: exit %d", code)
	}
}
