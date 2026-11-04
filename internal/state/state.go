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
// The root is always ~/.dongle: there is deliberately no environment
// variable to relocate it. (Tests isolate themselves by pointing the home
// directory at a temp dir; see internal/testutil.Isolate.)
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
	// with -tags embed; see internal/bootstrap/bootstrap.go) has unpacked its embedded
	// defaults into Plugins, so that only ever happens on first run.
	DefaultsBootstrapped bool `json:"defaultsBootstrapped,omitempty"`
}

func dataDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".dongle")
}

func DataDir() string    { return dataDir() }
func PluginsDir() string { return filepath.Join(dataDir(), "plugins") }

func PluginVersionDir(name, version string) string {
	return filepath.Join(PluginsDir(), name, version)
}

// EntrypointName is the file name a plugin's binary is installed under in
// its version dir: "dongle-<plugin>", whatever the host binary itself is
// called on disk (e.g. a release's dongle-linux-amd64). Both `dongle
// install` and the embedded defaults use it; dispatch reads the name back
// from state.json rather than recomputing it, so changing this never
// strands plugins that are already installed.
func EntrypointName(plugin string) string { return "dongle-" + plugin }

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
