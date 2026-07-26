// Package pluginsdk helps Go plugins read what the dongle host injects. A plugin
// is just an executable; the host passes context via environment variables. This
// SDK reads that contract so plugins don't parse it by hand. Plugins in other
// languages read the same env vars.
package pluginsdk

import "os"

// Context is the per-invocation info the host passes to every plugin.
type Context struct {
	HostVersion string
	Protocol    string
	PluginName  string
	Output      string // "text" | "json"
}

// LoadContext reads the context contract from the environment.
func LoadContext() Context {
	return Context{
		HostVersion: os.Getenv("DONGLE_VERSION"),
		Protocol:    os.Getenv("DONGLE_PROTOCOL"),
		PluginName:  os.Getenv("DONGLE_PLUGIN_NAME"),
		Output:      envOr("DONGLE_OUTPUT", "text"),
	}
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
