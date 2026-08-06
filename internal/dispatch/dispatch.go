// Package dispatch resolves an unknown top-level command to an installed plugin,
// checks compatibility, and execs the plugin as a one-shot child process (no
// gRPC, no persistent service).
package dispatch

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/andreimladin/dongle/internal/compat"
	"github.com/andreimladin/dongle/internal/state"
)

// Run executes the plugin registered under name with args, returning the
// resulting process exit code (or a nonzero host error code).
func Run(hostVersion, protocol, name string, args []string) int {
	st, err := state.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error: reading state:", err)
		return 1
	}
	inst, ok := st.Plugins[name]
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown command %q (try `dongle plugin list`)\n", name)
		return 127
	}

	verDir := state.PluginVersionDir(name, inst.ActiveVersion)

	// Compatibility gate (also enforced at install time; re-checked here because
	// the host binary can be upgraded after a plugin was installed).
	if ok, reason, err := compat.Check(hostVersion, protocol, inst.Requires); err != nil {
		fmt.Fprintln(os.Stderr, "error: bad constraint in state:", err)
		return 1
	} else if !ok {
		fmt.Fprintf(os.Stderr, "error: %s %s — upgrade dongle\n", name, reason)
		return 1
	}

	bin := filepath.Join(verDir, inst.Entrypoint)
	cmd := exec.Command(bin, args...)
	// Inherit the terminal so the plugin's prompts, colors, and progress work.
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	// The stable context contract every plugin can rely on.
	cmd.Env = append(os.Environ(),
		"DONGLE_VERSION="+hostVersion,
		"DONGLE_PROTOCOL="+protocol,
		"DONGLE_PLUGIN_NAME="+name,
	)

	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return ee.ExitCode() // plugin ran and chose to fail — pass its code through
		}
		fmt.Fprintf(os.Stderr, "error launching %s: %v\n", name, err)
		return 1
	}
	return 0
}
