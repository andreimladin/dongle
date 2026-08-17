// Package state tracks installed plugins and the on-disk layout:
//
//	<dataDir>/plugins/<name>/<version>/<entrypoint>
//	<dataDir>/state.json
//	<dataDir>/index/            (the cloned index cache; owned by package index)
//
// There is no per-plugin manifest file on disk: the plugin's manifest is
// resolved from the index at install time, and its runtime-relevant facts
// (entrypoint, requires) are recorded into state.json so dispatch never has
// to read anything but this file.
//
// Set DONGLE_DATA_DIR to override the root (used in tests so we don't touch the
// real ~/.dongle).
package state

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/andreimladin/dongle/internal/compat"
)

type Installed struct {
	Name          string          `json:"name"`
	ActiveVersion string          `json:"activeVersion"`
	Entrypoint    string          `json:"entrypoint"`
	Requires      compat.Requires `json:"requires"`
}

type State struct {
	Plugins map[string]Installed `json:"plugins"`
	// DefaultsBootstrapped is set once a batteries-included binary (built
	// with -tags embed; see cmd/embed.go) has unpacked its embedded
	// defaults into Plugins, so that only ever happens on first run.
	DefaultsBootstrapped bool `json:"defaultsBootstrapped,omitempty"`
}

func dataDir() string {
	if d := os.Getenv("DONGLE_DATA_DIR"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".dongle")
}

func DataDir() string    { return dataDir() }
func PluginsDir() string { return filepath.Join(dataDir(), "plugins") }

func PluginVersionDir(name, version string) string {
	return filepath.Join(PluginsDir(), name, version)
}

func statePath() string { return filepath.Join(dataDir(), "state.json") }

func Load() (*State, error) {
	b, err := os.ReadFile(statePath())
	if os.IsNotExist(err) {
		return &State{Plugins: map[string]Installed{}}, nil
	}
	if err != nil {
		return nil, err
	}
	var s State
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, err
	}
	if s.Plugins == nil {
		s.Plugins = map[string]Installed{}
	}
	return &s, nil
}

func (s *State) Save() error {
	if err := os.MkdirAll(dataDir(), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(statePath(), b, 0o644)
}
