// Package state tracks which plugins are installed and where they live on disk.
//
// Layout (XDG):
//
//	$XDG_DATA_HOME/dongle/
//	  plugins/<name>/<version>/{<entrypoint binary>, plugin.json}
//	  state.json
//
// Set DONGLE_DATA_DIR to override the root (used by demo.sh so tests don't touch
// your real data dir).
package state

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Installed records the active version of one plugin.
type Installed struct {
	Name          string `json:"name"`
	ActiveVersion string `json:"activeVersion"`
}

// State is the installed-plugin registry, keyed by plugin name.
type State struct {
	Plugins map[string]Installed `json:"plugins"`
}

func dataDir() string {
	if d := os.Getenv("DONGLE_DATA_DIR"); d != "" {
		return d
	}
	if x := os.Getenv("XDG_DATA_HOME"); x != "" {
		return filepath.Join(x, "dongle")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "dongle")
}

// DataDir is the root install directory.
func DataDir() string { return dataDir() }

// PluginsDir is where per-plugin trees live.
func PluginsDir() string { return filepath.Join(dataDir(), "plugins") }

// PluginVersionDir is .../plugins/<name>/<version>.
func PluginVersionDir(name, version string) string {
	return filepath.Join(PluginsDir(), name, version)
}

func statePath() string { return filepath.Join(dataDir(), "state.json") }

// Load returns the current state, or an empty one if nothing is installed yet.
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

// Save persists the state.
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
