// Package manifest describes a plugin and loads the plugin.json that ships
// inside each plugin's release artifact, next to the entrypoint binary.
package manifest

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/andreimladin/dongle/internal/compat"
)

type Manifest struct {
	Name        string          `json:"name"`        // registers `dongle <name>`
	Version     string          `json:"version"`     // plugin's own semver
	Description string          `json:"description"`
	Entrypoint  string          `json:"entrypoint"`  // binary filename in the version dir
	Requires    compat.Requires `json:"requires"`    // host + protocol gate
	Auth        []AuthEntry     `json:"auth"`        // credentials the host must broker
}

// AuthEntry declares one credential the host brokers to the plugin. The plugin
// never obtains it; it only consumes what the host injects.
type AuthEntry struct {
	Provider string   `json:"provider"`           // which credential, e.g. "platform-oidc"
	Scopes   []string `json:"scopes,omitempty"`   // for display / least-privilege
	InjectAs string   `json:"injectAs,omitempty"` // "file" (default) | "env"
	EnvVar   string   `json:"envVar,omitempty"`   // env name when InjectAs == "env"
}

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
		return nil, fmt.Errorf("manifest %s: name and entrypoint required", path)
	}
	return &m, nil
}
