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
downloaded from the feed. Once installed, run it as ` + "`dongle <name>`" + `.
` + indexCheckHelp,
	Example: `  dongle install deploy
  dongle install deploy --sync      # refresh the index first, no prompt
  dongle install deploy --no-sync   # use the cached index as-is`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return exitCode(plugincmd.Install(hostVersion, protocol, args[0], syncMode(cmd)))
	},
}

// indexCheckHelp is the shared --help paragraph for the commands that
// check the feed for a newer index before acting.
const indexCheckHelp = `
Before acting, the feed is checked for a newer plugin index than the cached
one. If there is one, you're asked whether to update the index first; when
not running in a terminal, the cached index is used and a note says a newer
one is available. --sync / --no-sync make that choice up front.`

// addSyncFlags registers the --sync/--no-sync pair on cmd.
func addSyncFlags(cmd *cobra.Command) {
	cmd.Flags().Bool("sync", false, "update the plugin index first if a newer one is available (no prompt)")
	cmd.Flags().Bool("no-sync", false, "use the cached plugin index without checking for a newer one")
	cmd.MarkFlagsMutuallyExclusive("sync", "no-sync")
}

// syncMode maps cmd's --sync/--no-sync flags to a plugincmd.SyncMode.
func syncMode(cmd *cobra.Command) plugincmd.SyncMode {
	if on, _ := cmd.Flags().GetBool("sync"); on {
		return plugincmd.SyncAlways
	}
	if off, _ := cmd.Flags().GetBool("no-sync"); off {
		return plugincmd.SyncNever
	}
	return plugincmd.SyncAsk
}

func init() {
	addSyncFlags(installCmd)
	addSyncFlags(upgradeCmd)
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
installed version is newer than the index's is never downgraded.
` + indexCheckHelp,
	Example: `  dongle upgrade            # upgrade everything installed
  dongle upgrade deploy     # upgrade just one plugin
  dongle upgrade --sync     # refresh the index first, no prompt`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := ""
		if len(args) == 1 {
			name = args[0]
		}
		return exitCode(plugincmd.Upgrade(hostVersion, protocol, name, syncMode(cmd)))
	},
}
