package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/andreimladin/dongle/internal/bootstrap"
	"github.com/andreimladin/dongle/internal/dispatch"
	"github.com/andreimladin/dongle/internal/hostcmd"
	"github.com/andreimladin/dongle/internal/index"
)

// Build-time build inputs, injected via -ldflags at build time (see
// scripts/build.sh and configs/build.yaml — the human-edited source of
// truth that script bakes these values from). A plain `go build ./cmd`
// leaves them at these defaults: hostVersion "dev", no index feed identity
// (DONGLE_INDEX_ORG/DONGLE_INDEX_FEED/DONGLE_INDEX_PACKAGE are then
// required to use `dongle refresh`/`dongle plugin` commands).
var (
	hostVersion  = "dev"
	indexOrg     = ""             // injected at build; env DONGLE_INDEX_ORG overrides
	indexProject = ""             // injected at build; env DONGLE_INDEX_PROJECT overrides; empty means an org-scoped feed
	indexFeed    = ""             // injected at build; env DONGLE_INDEX_FEED overrides
	indexPackage = "dongle-index" // injected at build; env DONGLE_INDEX_PACKAGE overrides
)

// protocol is the host<->plugin contract version. It is not a build
// input — it changes only when the connector spec itself changes — so it
// stays a const rather than joining the vars above.
const protocol = "v1"

// exitCode ends the process with an internal/* function's exit code. It is
// the single place dongle exits from a command: every cobra command's Run
// is a one-line adapter that calls its internal function and hands the
// returned code here. os.Exit skips deferred functions and cobra's
// post-run hooks, so internal functions finish their own cleanup before
// returning a code, and no command uses PostRun/PersistentPostRun.
func exitCode(code int) { os.Exit(code) }

var rootCmd = &cobra.Command{
	Use:   "dongle",
	Short: "dongle — one CLI, plug in the rest",
	Long: `dongle — one CLI, plug in the rest

Builtins:
  dongle version                   print the CLI version and the cached index version
  dongle refresh                   force-download the latest plugin index from the feed
  dongle plugin search             list plugins available in the index
  dongle plugin install <name>     install a plugin from the index
  dongle plugin list               list installed plugins
  dongle plugin update <name>      update an installed plugin to the index's current version
  dongle plugin uninstall <name>
  dongle support <plugin-name>     show where to get help with a plugin
  dongle <name> [args...]          run an installed plugin`,
	SilenceErrors: true,
	SilenceUsage:  true,
	// Anything not matched to a builtin subcommand by cobra's Find lands
	// here as a candidate plugin invocation. DisableFlagParsing keeps cobra
	// from trying to interpret the plugin's own flags — args are handed to
	// dispatch.Run completely unparsed.
	DisableFlagParsing: true,
	Args:               cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		// Bare `dongle` or -h/--help (not parsed by cobra here, see
		// DisableFlagParsing) renders cobra's help; anything else is a
		// plugin name, resolved and exec'd by dispatch.
		if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
			_ = cmd.Help()
			return
		}
		exitCode(dispatch.Run(hostVersion, protocol, args[0], args[1:]))
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "print the CLI version and the cached index version",
	Run:   func(cmd *cobra.Command, args []string) { exitCode(hostcmd.Version(hostVersion, protocol, args)) },
}

func init() {
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.SetFlagErrorFunc(func(c *cobra.Command, err error) error {
		fmt.Fprintln(os.Stderr, "error:", err)
		exitCode(2)
		return err
	})
	rootCmd.AddCommand(versionCmd, refreshCmd, pluginCmd, supportCmd)
}

// Execute runs the root command and returns the process exit code.
func Execute() int {
	// Wire the build-time-injected index feed identity (above) into
	// internal/index before any refresh/plugin command runs.
	index.SetDefaults(indexOrg, indexProject, indexFeed, indexPackage)

	// Batteries-included binaries (built with -tags embed) self-register
	// their embedded defaults here, before any dispatch happens. Plain
	// builds get the no-op in internal/bootstrap/noop.go.
	bootstrap.InstallDefaults()

	// Commands normally exit via exitCode from their own Run; reaching
	// here means cobra handled the invocation itself (e.g. --help, or bare
	// `dongle`), or failed before any Run was called.
	if err := rootCmd.Execute(); err != nil {
		return 1
	}
	return 0
}
