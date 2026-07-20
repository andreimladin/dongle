// Package manifest defines the local plugin manifest and how to load it.
//
// The design spec writes this as plugin.yaml; this scaffold uses JSON so it
// builds with zero external dependencies. To use YAML instead, import
// gopkg.in/yaml.v3 and replace the json.Unmarshal call in Load — nothing else
// changes.
package manifest

import (
	"encoding/json"
	"fmt"
	"os"
)

// Manifest ships next to each plugin binary and is read by the host at
// dispatch time.
type Manifest struct {
	APIVersion  string      `json:"apiVersion"` // protocol id, e.g. "dongle.plugin/v1"
	Name        string      `json:"name"`       // command it registers: `dongle <name>`
	Version     string      `json:"version"`    // plugin's own semver
	Description string      `json:"description"`
	Entrypoint  string      `json:"entrypoint"` // binary filename inside the version dir
	Requires    Requires    `json:"requires"`
	Auth        []AuthEntry `json:"auth"`
	Commands    []Command   `json:"commands"`
}

// Requires binds the three version axes: this plugin needs a host that
// satisfies Host and speaks Protocol.
type Requires struct {
	Host     string `json:"host"`     // semver constraint, e.g. ">=2.0.0"
	Protocol string `json:"protocol"` // protocol id, e.g. "v1"
}

// AuthEntry declares one credential the host must broker to the plugin.
type AuthEntry struct {
	Provider string   `json:"provider"`
	Scopes   []string `json:"scopes"`
	InjectAs string   `json:"injectAs"` // "file" (default) | "env"
	EnvVar   string   `json:"envVar"`   // env name to use when InjectAs == "env"
}

// Command is help/completion metadata so the host can render top-level help
// without exec-ing every plugin.
type Command struct {
	Name  string `json:"name"`
	Short string `json:"short"`
}

// Load reads and validates a manifest file.
func Load(path string) (*Manifest, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	var m Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("parse manifest %s: %w", path, err)
	}
	if m.Name == "" || m.Entrypoint == "" {
		return nil, fmt.Errorf("manifest %s: fields 'name' and 'entrypoint' are required", path)
	}
	return &m, nil
}
