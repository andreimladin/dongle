package main

import (
	"github.com/spf13/cobra"

	"github.com/andreimladin/dongle/internal/plugincmd"
)

// pluginCmd's own RunE only fires when cobra's Find couldn't match a deeper
// subcommand (no subcommand given, or an unrecognized one) — the known
// subcommands below are matched and handled directly by cobra. Either way
// the args are handed to plugincmd.Run exactly as the old hand-rolled
// switch did, so usage text and exit codes are unchanged.
var pluginCmd = &cobra.Command{
	Use:   "plugin",
	Short: "manage plugins",
	Args:  cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return exitCode(plugincmd.Run(hostVersion, protocol, args))
	},
}

var pluginListCmd = &cobra.Command{
	Use:   "list",
	Short: "list installed plugins",
	RunE: func(cmd *cobra.Command, args []string) error {
		return exitCode(plugincmd.Run(hostVersion, protocol, []string{"list"}))
	},
}

var pluginSearchCmd = &cobra.Command{
	Use:   "search",
	Short: "list plugins available in the index",
	RunE: func(cmd *cobra.Command, args []string) error {
		return exitCode(plugincmd.Run(hostVersion, protocol, []string{"search"}))
	},
}

var pluginInstallCmd = &cobra.Command{
	Use:   "install <name>",
	Short: "install a plugin from the index",
	RunE: func(cmd *cobra.Command, args []string) error {
		return exitCode(plugincmd.Run(hostVersion, protocol, append([]string{"install"}, args...)))
	},
}

var pluginUninstallCmd = &cobra.Command{
	Use:   "uninstall <name>",
	Short: "uninstall a plugin",
	RunE: func(cmd *cobra.Command, args []string) error {
		return exitCode(plugincmd.Run(hostVersion, protocol, append([]string{"uninstall"}, args...)))
	},
}

func init() {
	pluginCmd.AddCommand(pluginListCmd, pluginSearchCmd, pluginInstallCmd, pluginUninstallCmd)
}
