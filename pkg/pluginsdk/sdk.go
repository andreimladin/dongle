// Package pluginsdk helps Go plugins read what the dongle host injects. A plugin
// is just an executable; the host passes context + brokered credentials via env
// and an optional credentials file. This SDK reads that contract so plugins
// don't parse it by hand. Plugins in other languages read the same env/file.
package pluginsdk

import (
	"encoding/json"
	"os"
)

// Context is the per-invocation info the host passes to every plugin.
type Context struct {
	HostVersion  string
	Protocol     string
	PluginName   string
	InvocationID string
	Output       string // "text" | "json"
}

// LoadContext reads the context contract from the environment.
func LoadContext() Context {
	return Context{
		HostVersion:  os.Getenv("DONGLE_VERSION"),
		Protocol:     os.Getenv("DONGLE_PROTOCOL"),
		PluginName:   os.Getenv("DONGLE_PLUGIN_NAME"),
		InvocationID: os.Getenv("DONGLE_INVOCATION_ID"),
		Output:       envOr("DONGLE_OUTPUT", "text"),
	}
}

// Token returns a credential the host brokered for a provider. It reads the
// injected credentials file first (injectAs:"file"), then falls back to the
// default env var DONGLE_TOKEN_<provider>. For a custom envVar (injectAs:"env"
// with your own name), read that env var directly instead.
func Token(provider string) (string, bool) {
	if path := os.Getenv("DONGLE_CREDENTIALS_FILE"); path != "" {
		if b, err := os.ReadFile(path); err == nil {
			var doc struct {
				Tokens map[string]string `json:"tokens"`
			}
			if json.Unmarshal(b, &doc) == nil {
				if t, ok := doc.Tokens[provider]; ok {
					return t, true
				}
			}
		}
	}
	if t := os.Getenv("DONGLE_TOKEN_" + provider); t != "" {
		return t, true
	}
	return "", false
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
