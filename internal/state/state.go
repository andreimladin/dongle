// Package state tracks installed plugins and the on-disk layout:
//
//	<dataDir>/plugins/<name>/<version>/{<entrypoint>, plugin.json}
//	<dataDir>/state.json
//	<dataDir>/index/            (the cloned index cache; owned by package index)
//	<dataDir>/credentials.json  (the credential store; owned by package auth)
//
// Set DONGLE_DATA_DIR to override the root (used in tests so we don't touch the
// real ~/.dongle).
package state

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Installed struct {
	Name          string `json:"name"`
	ActiveVersion string `json:"activeVersion"`
}

type State struct {
	Plugins map[string]Installed `json:"plugins"`
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
