package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/andreimladin/dongle/internal/dispatch"
)

// hostVersion is the host's own semver. It defaults to "dev" for a plain
// `go build`; scripts/build.sh and scripts/build-release.sh stamp the real
// version in via -ldflags "-X main.hostVersion=...".
var hostVersion = "dev"

const protocol = "v1" // the host<->plugin contract version

// exitError carries a specific process exit code through cobra's error
// return path. The command that produced it has already printed its own
// "error: ..." message to stderr (matching the pre-cobra behavior), so
// exitError itself carries no message — cobra must not print anything for
// it (see SilenceErrors below).
type exitError struct{ code int }

func (e *exitError) Error() string { return "" }

// exitCode turns an internal/*'s int exit code into an error cobra can
// carry back to Execute, or nil for success.
func exitCode(code int) error {
	if code == 0 {
		return nil
	}
	return &exitError{code: code}
}

var rootCmd = &cobra.Command{
	Use:   "dongle",
	Short: "dongle — one CLI, plug in the rest",
	Long: `dongle — one CLI, plug in the rest

Builtins:
  dongle version
  dongle plugin search             list plugins available in the index
  dongle plugin install <name>     install a plugin from the index
  dongle plugin list               list installed plugins
  dongle plugin uninstall <name>
  dongle index refresh             force-refresh the index cache
  dongle index status
  dongle <name> [args...]          run an installed plugin`,
	SilenceErrors: true,
	SilenceUsage:  true,
	// Anything not matched to a builtin subcommand by cobra's Find lands
	// here as a candidate plugin invocation. DisableFlagParsing keeps cobra
	// from trying to interpret the plugin's own flags — args are handed to
	// dispatch.Run completely unparsed.
	DisableFlagParsing: true,
	Args:               cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}
		switch args[0] {
		case "-h", "--help":
			return cmd.Help()
		default:
			// Not a builtin -> resolve to an installed plugin and exec it.
			return exitCode(dispatch.Run(hostVersion, protocol, args[0], args[1:]))
		}
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "print the host version",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("dongle %s (protocol %s)\n", hostVersion, protocol)
		return nil
	},
}

func init() {
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.SetFlagErrorFunc(func(c *cobra.Command, err error) error {
		fmt.Fprintln(os.Stderr, "error:", err)
		return exitCode(2)
	})
	rootCmd.AddCommand(versionCmd, pluginCmd, indexCmd)
}

// Execute runs the root command and returns the process exit code.
func Execute() int {
	// Batteries-included binaries (built with -tags embed) self-register
	// their embedded defaults here, before any dispatch happens. Plain
	// builds get the no-op in embed_noop.go.
	installEmbeddedDefaults()

	if err := rootCmd.Execute(); err != nil {
		var ee *exitError
		if errors.As(err, &ee) {
			return ee.code
		}
		return 1
	}
	return 0
}
