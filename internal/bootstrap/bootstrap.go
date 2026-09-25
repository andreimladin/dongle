//go:build embed

// Package bootstrap unpacks what's baked into a "batteries-included"
// release binary (see configs/build.yaml and scripts/build.sh): the
// default plugins, into the normal plugin store on first run, and the
// plugin index archive, exposed to internal/index (see EmbeddedIndex) so
// it can seed its cache offline on a first run with nothing cached yet.
// Everything the //go:embed directive needs — the directive itself and the
// staged embedded/ payload — lives here because go:embed paths are
// relative to the source file and can't reach outside this package with
// "../".
package bootstrap

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"

	"github.com/andreimladin/dongle/internal/state"
)

// embeddedFS holds whatever scripts/build.sh's fetch_embedded staged into
// internal/bootstrap/embedded/ (plugin binaries + manifest.json) before
// this file's package was compiled with -tags embed. The tracked .gitkeep
// keeps the directory non-empty for plain `go build -tags embed`, so
// embeddedFS is always valid even when nothing was staged — the "all:"
// prefix is needed so go:embed doesn't reject the directory for containing
// only a dot-prefixed file.
//
//go:embed all:embedded
var embeddedFS embed.FS

// embeddedManifest is the shape of embedded/manifest.json: the plugin
// defaults staged for this build, plus the version of the index archive
// staged alongside them as embedded/index.tar.gz (see EmbeddedIndex).
type embeddedManifest struct {
	Plugins []Default         `json:"plugins"`
	Index   embeddedIndexMeta `json:"index"`
}

// embeddedIndexMeta records the version of the plugin index staged as
// embedded/index.tar.gz. Version is empty when fetch_embedded staged no
// index for this build.
type embeddedIndexMeta struct {
	Version string `json:"version"`
}

// loadManifest reads and parses embedded/manifest.json. ok is false when
// nothing was staged for this build (a plain `go build -tags embed` with
// an empty internal/bootstrap/embedded) or the manifest is invalid.
func loadManifest() (m embeddedManifest, ok bool) {
	b, err := fs.ReadFile(embeddedFS, "embedded/manifest.json")
	if err != nil {
		return embeddedManifest{}, false // nothing staged for this build
	}
	if err := json.Unmarshal(b, &m); err != nil {
		fmt.Fprintln(os.Stderr, "warning: embedded defaults manifest is invalid:", err)
		return embeddedManifest{}, false
	}
	return m, true
}

// PendingDefaults returns the default plugins baked into this binary (see
// configs/build.yaml and scripts/build.sh) that still need unpacking into
// the plugin store — all of them on first run, none afterwards. It returns
// nil when nothing was staged (a plain `go build -tags embed` with an
// empty internal/bootstrap/embedded) or the bootstrap already happened.
// The caller installs each with InstallDefault, then calls
// MarkDefaultsBootstrapped so this never runs again.
func PendingDefaults() []Default {
	manifest, ok := loadManifest()
	if !ok || len(manifest.Plugins) == 0 {
		return nil
	}
	st, err := state.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "warning: could not load state for embedded defaults:", err)
		return nil
	}
	if st.DefaultsBootstrapped {
		return nil
	}
	return manifest.Plugins
}

// InstallDefault writes one embedded default plugin to its canonical
// on-disk location and records it in state.
func InstallDefault(d Default) error {
	st, err := state.Load()
	if err != nil {
		return err
	}
	if err := installEmbeddedDefault(st, d); err != nil {
		return err
	}
	return st.Save()
}

// MarkDefaultsBootstrapped records that the embedded defaults have been
// unpacked, so PendingDefaults reports none from now on.
func MarkDefaultsBootstrapped() error {
	st, err := state.Load()
	if err != nil {
		return err
	}
	st.DefaultsBootstrapped = true
	return st.Save()
}

// EmbeddedIndex returns the plugin-index archive baked into this binary by
// scripts/build.sh's fetch_embedded (internal/bootstrap/embedded/index.tar.gz)
// and the version recorded for it in embedded/manifest.json, for
// internal/index to seed its cache from on a first run with nothing cached
// yet — entirely offline, no feed call. ok is false when nothing was
// embedded (a plain `go build -tags embed` with an empty
// internal/bootstrap/embedded), mirroring PendingDefaults' own no-op case.
func EmbeddedIndex() (archive []byte, version string, ok bool) {
	manifest, mok := loadManifest()
	if !mok || manifest.Index.Version == "" {
		return nil, "", false
	}
	data, err := fs.ReadFile(embeddedFS, "embedded/index.tar.gz")
	if err != nil {
		return nil, "", false
	}
	return data, manifest.Index.Version, true
}

// installEmbeddedDefault writes one staged plugin binary to its canonical
// on-disk location and records it in st.Plugins; the caller saves st.
func installEmbeddedDefault(st *state.State, d Default) error {
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
// same convention as plugins installed via `dongle install`.
func hostBinaryName() string {
	name := filepath.Base(os.Args[0])
	if name == "" || name == "." || name == string(filepath.Separator) {
		return "dongle"
	}
	return name
}
