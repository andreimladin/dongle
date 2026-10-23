//go:build !embed

package bootstrap

import (
	"os"
	"testing"

	"github.com/andreimladin/dongle/internal/state"
)

// A plain build embeds nothing: first-run initialization must find nothing
// to do and must not write anything to the data dir.
func TestPlainBuildEmbedsNothing(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DONGLE_DATA_DIR", dir)

	if d := PendingDefaults(); len(d) != 0 {
		t.Fatalf("PendingDefaults() = %v, want none", d)
	}
	if archive, version, ok := EmbeddedIndex(); ok || archive != nil || version != "" {
		t.Fatalf("EmbeddedIndex() = %d bytes, %q, %v; want nothing", len(archive), version, ok)
	}
	if err := InstallDefault(Default{Name: "deploy", Version: "1.0.0"}); err != nil {
		t.Fatal(err)
	}
	if err := MarkDefaultsBootstrapped(); err != nil {
		t.Fatal(err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Fatalf("plain build wrote to the data dir: %v", entries)
	}
	if st, _ := state.Load(); len(st.Plugins) != 0 || st.DefaultsBootstrapped {
		t.Fatalf("state changed: %+v", st)
	}
}
