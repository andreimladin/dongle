package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/andreimladin/dongle/internal/builtins"
	"github.com/andreimladin/dongle/internal/dispatch"
	"github.com/andreimladin/dongle/internal/index"
	"github.com/andreimladin/dongle/internal/ui"
)

// Build-time build inputs, injected via -ldflags at build time (see
// scripts/build.sh and configs/build.yaml — the human-edited source of
// truth that script bakes these values from). A plain `go build ./cmd`
// leaves them at these defaults: hostVersion "dev", no index feed identity
// (DONGLE_INDEX_ORG/DONGLE_INDEX_FEED/DONGLE_INDEX_PACKAGE are then
// required to use `dongle sync` and the index-reading commands).
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

dongle is a host CLI that other CLIs plug into. Besides the builtin
commands below, any installed plugin runs as a top-level command:

  dongle <plugin> [args...]

Everything after the plugin name is passed to it unchanged.`,
	Example: `  dongle search            # what can I install?
  dongle install deploy    # install a plugin
  dongle deploy --help     # run it
  dongle upgrade           # upgrade everything installed
  dongle --version         # host, index and plugin versions`,
	SilenceErrors: true,
	SilenceUsage:  true,
	// Anything not matched to a builtin subcommand by cobra's Find lands
	// here as a candidate plugin invocation. DisableFlagParsing keeps cobra
	// from trying to interpret the plugin's own flags — args are handed to
	// dispatch.Run completely unparsed. It also means the root's own
	// --help/--version flags arrive here as plain args (see below).
	DisableFlagParsing: true,
	Args:               cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}
		switch args[0] {
		case "-h", "--help":
			return cmd.Help()
		case "--version":
			return exitCode(builtins.Version(hostVersion))
		}
		return runPlugin(cmd, args)
	},
}

// runPlugin applies the fall-through rule for a name cobra didn't match to
// a builtin: an installed plugin is dispatched (args passed through
// unchanged); anything else is an error followed by the root help.
func runPlugin(cmd *cobra.Command, args []string) error {
	name := args[0]
	if strings.HasPrefix(name, "-") {
		ui.Errorf("unknown flag %q\n", name)
		return usageError(cmd)
	}
	installed, err := dispatch.IsInstalled(name)
	if err != nil {
		ui.Errorf("reading state: %v", err)
		return exitCode(1)
	}
	if installed {
		return exitCode(dispatch.Run(hostVersion, protocol, name, args[1:]))
	}
	ui.Errorf("command %q is not supported", name)
	if s := cmd.SuggestionsFor(name); len(s) > 0 {
		fmt.Fprintf(os.Stderr, "Did you mean: %s?\n", strings.Join(s, ", "))
	}
	fmt.Fprintln(os.Stderr)
	return usageError(cmd)
}

// usageError prints the root help to stderr (it's a diagnostic here, not
// the command's output) and returns the usage-error exit code.
func usageError(cmd *cobra.Command) error {
	root := cmd.Root()
	root.SetOut(os.Stderr)
	_ = root.Help()
	root.SetOut(nil)
	return exitCode(2)
}

// helpCmd displaces cobra's built-in `help` command: dongle's help is
// --help/-h only. cobra always registers *some* help command on a root
// with subcommands, so this hidden one takes the slot under a name nobody
// types (cobra's templates special-case the literal name "help", so it
// can't be that). `dongle help` itself then matches no builtin and goes
// through the root's builtin-or-plugin resolution like any other name — an
// error plus the root help, unless a plugin named "help" is installed.
var helpCmd = &cobra.Command{
	Use:                "__help",
	Hidden:             true,
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runPlugin(cmd.Root(), append([]string{cmd.Name()}, args...))
	},
}

// usageTemplate is cobra's default usage template with the root's usage
// lines replaced to describe both ways dongle is invoked; subcommands keep
// cobra's default lines.
const usageTemplate = `Usage:{{if not .HasParent}}
  {{.CommandPath}} <command> [flags]
  {{.CommandPath}} <plugin> [args...]{{else}}{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{end}}{{if gt (len .Aliases) 0}}
`

func init() {
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.SetHelpCommand(helpCmd)
	def := rootCmd.UsageTemplate()
	rootCmd.SetUsageTemplate(usageTemplate + def[strings.Index(def, "\nAliases:"):])
	// --version is handled by hand in rootCmd.RunE (flag parsing is off on
	// the root); it's declared here only so it's listed in --help.
	rootCmd.Flags().Bool("version", false, "print dongle, index and installed plugin versions")
	rootCmd.SetFlagErrorFunc(func(c *cobra.Command, err error) error {
		ui.Errorf("%v", err)
		fmt.Fprintf(os.Stderr, "Run '%s --help' for usage.\n", c.CommandPath())
		return exitCode(2)
	})
	rootCmd.AddCommand(searchCmd, installCmd, removeCmd, upgradeCmd, syncCmd, supportCmd)
}

// Execute runs the root command and returns the process exit code.
func Execute() int {
	// Wire the build-time-injected index feed identity (above) into
	// internal/index before any refresh/plugin command runs.
	index.SetDefaults(indexOrg, indexProject, indexFeed, indexPackage)

	// Batteries-included binaries (built with -tags embed) unpack their
	// embedded index and default plugins here on first run, before any
	// dispatch happens. Plain builds embed nothing, so it's a no-op.
	builtins.Initialize()

	cmd, err := rootCmd.ExecuteC()
	if err != nil {
		var ee *exitError
		if errors.As(err, &ee) {
			return ee.code
		}
		// Anything else is cobra's own (e.g. wrong argument count); errors
		// are silenced on the root, so report it here.
		ui.Errorf("%v", err)
		fmt.Fprintf(os.Stderr, "Run '%s --help' for usage.\n", cmd.CommandPath())
		return 2
	}
	return 0
}
