package main

import (
	"github.com/spf13/cobra"

	"github.com/andreimladin/dongle/internal/plugincmd"
)

// Each command below is a one-line adapter: cobra's command tree routes to
// it, it calls the matching internal/plugincmd function, and exits with
// the returned code. Arg validation, output and errors all live in
// plugincmd.

// pluginCmd's own Run only fires when cobra's Find couldn't match a
// subcommand (none given, or an unrecognized one); ArbitraryArgs lets that
// case reach plugincmd.Fallback instead of a cobra error.
var pluginCmd = &cobra.Command{
	Use:   "plugin",
	Short: "manage plugins",
	Long:  "Manage installed plugins: search the index, install, list, update and uninstall.",
	Args:  cobra.ArbitraryArgs,
	Run:   func(cmd *cobra.Command, args []string) { exitCode(plugincmd.Fallback(args)) },
}

var pluginListCmd = &cobra.Command{
	Use:   "list",
	Short: "list installed plugins",
	Run:   func(cmd *cobra.Command, args []string) { exitCode(plugincmd.List(args)) },
}

var pluginSearchCmd = &cobra.Command{
	Use:   "search",
	Short: "list plugins available in the index",
	Run:   func(cmd *cobra.Command, args []string) { exitCode(plugincmd.Search(args)) },
}

var pluginInstallCmd = &cobra.Command{
	Use:   "install <name>",
	Short: "install a plugin from the index",
	Run:   func(cmd *cobra.Command, args []string) { exitCode(plugincmd.Install(hostVersion, protocol, args)) },
}

var pluginUninstallCmd = &cobra.Command{
	Use:   "uninstall <name>",
	Short: "uninstall a plugin",
	Run:   func(cmd *cobra.Command, args []string) { exitCode(plugincmd.Uninstall(args)) },
}

var pluginUpdateCmd = &cobra.Command{
	Use:   "update <name>",
	Short: "update an installed plugin to the index's current version",
	Run:   func(cmd *cobra.Command, args []string) { exitCode(plugincmd.Update(hostVersion, protocol, args)) },
}

func init() {
	pluginCmd.AddCommand(pluginListCmd, pluginSearchCmd, pluginInstallCmd, pluginUninstallCmd, pluginUpdateCmd)
}
