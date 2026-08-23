//go:build embed

// Package bootstrap unpacks the plugins baked into a "batteries-included"
// release binary (see defaults.lock and scripts/build-release.sh) into the
// normal plugin store on first run. Everything the //go:embed directive
// needs — the directive itself and the staged embedded/ payload — lives
// here because go:embed paths are relative to the source file and can't
// reach outside this package with "../".
package bootstrap

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"

	"github.com/andreimladin/dongle/internal/compat"
	"github.com/andreimladin/dongle/internal/state"
)

// embeddedFS holds whatever scripts/build-release.sh staged into
// internal/bootstrap/embedded/ (plugin binaries + manifest.json) before
// this file's package was compiled with -tags embed. The tracked .gitkeep
// keeps the directory non-empty for plain `go build -tags embed`, so
// embeddedFS is always valid even when nothing was staged — the "all:"
// prefix is needed so go:embed doesn't reject the directory for containing
// only a dot-prefixed file.
//
//go:embed all:embedded
var embeddedFS embed.FS

// embeddedDefault is one entry of embedded/manifest.json, written by
// scripts/build-release.sh from defaults.lock.
type embeddedDefault struct {
	Name       string          `json:"name"`
	Version    string          `json:"version"`
	File       string          `json:"file"`
	Entrypoint string          `json:"entrypoint,omitempty"`
	Requires   compat.Requires `json:"requires,omitempty"`
}

// InstallDefaults unpacks the plugins baked into this binary (see
// defaults.lock and scripts/build-release.sh) into the plugin store on
// first run only, then records that in state so it never runs again. It is
// a no-op when nothing was staged — either a plain `go build -tags embed`
// with an empty internal/bootstrap/embedded, or a run after the bootstrap
// flag is already set.
func InstallDefaults() {
	manifestBytes, err := fs.ReadFile(embeddedFS, "embedded/manifest.json")
	if err != nil {
		return // nothing staged for this build
	}
	var defaults []embeddedDefault
	if err := json.Unmarshal(manifestBytes, &defaults); err != nil {
		fmt.Fprintln(os.Stderr, "warning: embedded defaults manifest is invalid:", err)
		return
	}
	if len(defaults) == 0 {
		return
	}

	st, err := state.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "warning: could not load state for embedded defaults:", err)
		return
	}
	if st.DefaultsBootstrapped {
		return
	}

	for _, d := range defaults {
		if err := installEmbeddedDefault(st, d); err != nil {
			fmt.Fprintf(os.Stderr, "warning: installing embedded default %s: %v\n", d.Name, err)
		}
	}

	st.DefaultsBootstrapped = true
	if err := st.Save(); err != nil {
		fmt.Fprintln(os.Stderr, "warning: could not record embedded defaults bootstrap:", err)
	}
}

// installEmbeddedDefault writes one staged plugin binary to its canonical
// on-disk location and records it in st.Plugins. st is saved once by the
// caller after every entry has been placed.
func installEmbeddedDefault(st *state.State, d embeddedDefault) error {
	// embed.FS paths are always "/"-separated regardless of host OS.
	data, err := fs.ReadFile(embeddedFS, path.Join("embedded", d.File))
	if err != nil {
		return err
	}

	entrypoint := d.Entrypoint
	if entrypoint == "" {
		entrypoint = hostBinaryName() + "-" + d.Name
	}

	verDir := state.PluginVersionDir(d.Name, d.Version)
	if err := os.MkdirAll(verDir, 0o755); err != nil {
		return err
	}
	dst := filepath.Join(verDir, entrypoint)
	if err := os.WriteFile(dst, data, 0o755); err != nil {
		return err
	}
	if err := os.Chmod(dst, 0o755); err != nil {
		return err
	}

	st.Plugins[d.Name] = state.Installed{
		Name:          d.Name,
		ActiveVersion: d.Version,
		Entrypoint:    entrypoint,
		Requires:      d.Requires,
	}
	return nil
}

// hostBinaryName mirrors internal/plugincmd's canonical entrypoint naming
// (<host binary name>-<plugin name>) so embedded defaults land under the
// same convention as plugins installed via `dongle plugin install`.
func hostBinaryName() string {
	name := filepath.Base(os.Args[0])
	if name == "" || name == "." || name == string(filepath.Separator) {
		return "dongle"
	}
	return name
}
