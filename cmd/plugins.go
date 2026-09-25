package main

import (
	"github.com/spf13/cobra"

	"github.com/andreimladin/dongle/internal/plugincmd"
)

// The plugin-management builtins live directly at the root (there is no
// `plugin` parent group). Each is a thin adapter: argument-count
// validation is cobra's job, everything else — output, errors, exit codes
// — is internal/plugincmd's.

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List installed plugins",
	Long: `List the plugins installed on this machine and their active versions.

Reads local state only; never contacts the feed.`,
	Example: `  dongle list`,
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return exitCode(plugincmd.List())
	},
}

var searchCmd = &cobra.Command{
	Use:   "search",
	Short: "List plugins available in the index",
	Long: `List every plugin available in the plugin index, with its latest
version and description.

Uses the cached index, refreshing it from the feed first if it is older
than the cache TTL.`,
	Example: `  dongle search`,
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return exitCode(plugincmd.Search())
	},
}

var installCmd = &cobra.Command{
	Use:   "install <name>",
	Short: "Install a plugin from the index",
	Long: `Install a plugin from the plugin index.

The plugin's manifest is resolved from the index, checked for
compatibility with this dongle, and its binary for this OS/architecture is
downloaded from the feed. Once installed, run it as ` + "`dongle <name>`" + `.`,
	Example: `  dongle install deploy
  dongle deploy --help`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return exitCode(plugincmd.Install(hostVersion, protocol, args[0]))
	},
}

var removeCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove an installed plugin",
	Long: `Remove an installed plugin: deletes every installed version of it from
disk and drops it from local state.`,
	Example: `  dongle remove deploy`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return exitCode(plugincmd.Remove(args[0]))
	},
}

var upgradeCmd = &cobra.Command{
	Use:   "upgrade [name]",
	Short: "Upgrade installed plugins to the index's versions",
	Long: `Upgrade installed plugins to the versions declared in the plugin index.

With no argument, every installed plugin is upgraded; with a name, only
that plugin. Only installed plugins are touched, and a plugin whose
installed version is newer than the index's is never downgraded.`,
	Example: `  dongle upgrade          # upgrade everything installed
  dongle upgrade deploy   # upgrade just one plugin`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := ""
		if len(args) == 1 {
			name = args[0]
		}
		return exitCode(plugincmd.Upgrade(hostVersion, protocol, name))
	},
}
