// Package pluginsdk provides helpers for writing dongle plugins in Go.
//
// A plugin is just an executable; the host injects context and brokered
// credentials via env + a credentials file. This SDK reads them for you so a
// plugin never parses that contract by hand. Plugins in other languages simply
// read the same env vars and file — see README for the contract.
package pluginsdk

import (
	"encoding/json"
	"fmt"
	"os"
)

// Context is the per-invocation information the host passes to every plugin.
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

// Token returns a credential the host brokered for the given provider. It reads
// the injected credentials file first (injectAs:"file"), then falls back to the
// default env var DONGLE_TOKEN_<provider>. If your manifest set a custom envVar
// for an injectAs:"env" credential, read that env var directly instead.
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

// Main is a tiny subcommand dispatcher so simple plugins stay small. The key ""
// is the default handler (no subcommand). Plugins that want full flag parsing
// can ignore this and use cobra themselves.
func Main(handlers map[string]func(args []string) int) {
	cmd := ""
	var rest []string
	if len(os.Args) > 1 {
		cmd = os.Args[1]
		rest = os.Args[2:]
	}
	h, ok := handlers[cmd]
	if !ok {
		if h, ok = handlers[""]; !ok {
			fmt.Fprintf(os.Stderr, "unknown subcommand %q\n", cmd)
			os.Exit(2)
		}
	}
	os.Exit(h(rest))
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
