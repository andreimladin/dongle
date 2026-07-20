// Package dispatch resolves an unknown top-level command to an installed
// plugin, checks compatibility, brokers credentials, and execs the plugin as a
// one-shot process (no gRPC, no persistent service).
package dispatch

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/andreimladin/dongle/internal/auth"
	"github.com/andreimladin/dongle/internal/manifest"
	"github.com/andreimladin/dongle/internal/state"
)

// Run executes the plugin registered under name with the given args and returns
// the resulting process exit code (or a nonzero host error code).
func Run(hostVersion, protocol, name string, args []string) int {
	st, err := state.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error: reading state:", err)
		return 1
	}
	inst, ok := st.Plugins[name]
	if !ok {
		fmt.Fprintf(os.Stderr, "error: unknown command %q\n", name)
		fmt.Fprintf(os.Stderr, "hint: try `dongle plugin list`\n")
		return 127
	}

	verDir := state.PluginVersionDir(name, inst.ActiveVersion)
	m, err := manifest.Load(filepath.Join(verDir, "plugin.json"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}

	// Compatibility: does the running host satisfy what the plugin requires?
	if okHost, err := satisfiesHost(hostVersion, m.Requires.Host); err != nil {
		fmt.Fprintln(os.Stderr, "error: bad host constraint in manifest:", err)
		return 1
	} else if !okHost {
		fmt.Fprintf(os.Stderr, "error: %s requires host %s but this host is %s — upgrade dongle\n",
			name, m.Requires.Host, hostVersion)
		return 1
	}
	if m.Requires.Protocol != "" && m.Requires.Protocol != protocol {
		fmt.Fprintf(os.Stderr, "error: %s speaks protocol %q but this host supports %q\n",
			name, m.Requires.Protocol, protocol)
		return 1
	}

	invocationID := randHex(8)

	// Broker credentials into the plugin process.
	inj, err := auth.Prepare(m, invocationID)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	defer inj.Cleanup()

	bin := filepath.Join(verDir, m.Entrypoint)
	cmd := exec.Command(bin, args...)
	// Inherit the terminal so plugin prompts, colors, and progress bars work.
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// The stable context contract every plugin can rely on.
	cmd.Env = append(os.Environ(),
		"DONGLE_VERSION="+hostVersion,
		"DONGLE_PROTOCOL="+protocol,
		"DONGLE_PLUGIN_NAME="+name,
		"DONGLE_INVOCATION_ID="+invocationID,
		"DONGLE_OUTPUT="+envOr("DONGLE_OUTPUT", "text"),
	)
	cmd.Env = append(cmd.Env, inj.Env...)

	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return ee.ExitCode()
		}
		fmt.Fprintf(os.Stderr, "error: launching plugin %s: %v\n", name, err)
		return 1
	}
	return 0
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func randHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "00000000000000000000"[:n*2]
	}
	return hex.EncodeToString(b)
}

// satisfiesHost supports the constraint forms this scaffold needs: "" (any),
// ">=x.y.z", and exact "x.y.z". For full ranges (^, ~, <, ||) drop in a real
// semver library such as github.com/Masterminds/semver.
func satisfiesHost(hostVersion, constraint string) (bool, error) {
	constraint = strings.TrimSpace(constraint)
	if constraint == "" {
		return true, nil
	}
	hv, err := parseSemver(hostVersion)
	if err != nil {
		return false, err
	}
	if strings.HasPrefix(constraint, ">=") {
		cv, err := parseSemver(strings.TrimSpace(constraint[2:]))
		if err != nil {
			return false, err
		}
		return compareSemver(hv, cv) >= 0, nil
	}
	cv, err := parseSemver(constraint)
	if err != nil {
		return false, err
	}
	return compareSemver(hv, cv) == 0, nil
}

func parseSemver(s string) ([3]int, error) {
	var out [3]int
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	parts := strings.SplitN(s, ".", 3)
	if len(parts) != 3 {
		return out, fmt.Errorf("invalid semver %q", s)
	}
	for i := 0; i < 3; i++ {
		p := parts[i]
		if i == 2 { // strip -prerelease / +build from patch
			if j := strings.IndexAny(p, "-+"); j >= 0 {
				p = p[:j]
			}
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return out, fmt.Errorf("invalid semver %q", s)
		}
		out[i] = n
	}
	return out, nil
}

func compareSemver(a, b [3]int) int {
	for i := 0; i < 3; i++ {
		if a[i] != b[i] {
			if a[i] < b[i] {
				return -1
			}
			return 1
		}
	}
	return 0
}
