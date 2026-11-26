package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/andreimladin/dongle/internal/index"
	"github.com/andreimladin/dongle/internal/state"
	"github.com/andreimladin/dongle/internal/testutil"
)

// run executes dongle with args the way main does and returns the exit
// code and output. The data dir is a fresh temp dir per test and PATH is
// empty, so no real `az` (or feed) can be reached.
func run(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	if args == nil {
		args = []string{} // nil would make cobra fall back to os.Args
	}
	rootCmd.SetArgs(args)
	defer resetFlags(rootCmd)
	stdout, stderr = testutil.Capture(t, func() { code = Execute() })
	return code, stdout, stderr
}

// resetFlags undoes flag values left on the package-level cobra commands
// by a previous Execute, so tests don't leak --sync etc. into each other.
func resetFlags(c *cobra.Command) {
	c.Flags().VisitAll(func(f *pflag.Flag) {
		_ = f.Value.Set(f.DefValue)
		f.Changed = false
	})
	for _, sub := range c.Commands() {
		resetFlags(sub)
	}
}

func mustContain(t *testing.T, what, got string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(got, w) {
			t.Errorf("%s missing %q; got:\n%s", what, w, got)
		}
	}
}

// cacheIndex writes a fresh cached index listing one plugin.
func cacheIndex(t *testing.T) {
	t.Helper()
	dir := filepath.Join(state.DataDir(), "index", "plugins")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "deploy.yaml"), []byte("name: deploy\nversion: 1.0.0\nshortDescription: Ship it\n"+
		"support:\n  documentation: https://docs.example/deploy\n  channel: \"#deploy\"\n"), 0o644)
	os.WriteFile(filepath.Join(state.DataDir(), "index.version"), []byte("2024.1"), 0o644)
	if err := index.MarkChecked(); err != nil {
		t.Fatal(err)
	}
}
