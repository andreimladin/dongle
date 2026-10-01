package main

import (
	"github.com/spf13/cobra"

	"github.com/andreimladin/dongle/internal/builtins"
)

// The plugin-management builtins live directly at the root (there is no
// `plugin` parent group). Each is a thin adapter: argument-count
// validation is cobra's job, everything else — output, errors, exit codes
// — is internal/builtins'.

var searchCmd = &cobra.Command{
	Use:   "search",
	Short: "List plugins available in the index",
	Long:  "List every plugin in the index with its latest version and description.",
	Example: `  dongle search
  dongle install deploy     # then install one`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return exitCode(builtins.Search(syncMode(cmd)))
	},
}

var installCmd = &cobra.Command{
	Use:   "install <name>",
	Short: "Install a plugin from the index",
	Long:  "Install a plugin and make it available as a dongle command.",
	Example: `  dongle install deploy
  dongle deploy               # then run it`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return exitCode(builtins.Install(hostVersion, protocol, args[0], syncMode(cmd)))
	},
}

// addSyncFlags registers the --sync/--no-sync pair on cmd (whether to run
// the equivalent of `dongle update` first).
func addSyncFlags(cmd *cobra.Command) {
	cmd.Flags().Bool("sync", false, "update the plugin index first if a newer one is available (no prompt)")
	cmd.Flags().Bool("no-sync", false, "use the cached plugin index without checking for a newer one")
	cmd.MarkFlagsMutuallyExclusive("sync", "no-sync")
}

// syncMode maps cmd's --sync/--no-sync flags to a builtins.SyncMode.
func syncMode(cmd *cobra.Command) builtins.SyncMode {
	if on, _ := cmd.Flags().GetBool("sync"); on {
		return builtins.SyncAlways
	}
	if off, _ := cmd.Flags().GetBool("no-sync"); off {
		return builtins.SyncNever
	}
	return builtins.SyncAsk
}

func init() {
	addSyncFlags(searchCmd)
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
		return exitCode(builtins.Remove(args[0]))
	},
}

var upgradeCmd = &cobra.Command{
	Use:   "upgrade [name]",
	Short: "Upgrade installed plugins",
	Long:  "Upgrade one plugin, or all installed plugins when no name is given.",
	Example: `  dongle upgrade            # upgrade everything installed
  dongle upgrade deploy     # upgrade just one plugin`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := ""
		if len(args) == 1 {
			name = args[0]
		}
		return exitCode(builtins.Upgrade(hostVersion, protocol, name, syncMode(cmd)))
	},
}
