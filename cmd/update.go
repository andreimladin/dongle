package main

import (
	"github.com/spf13/cobra"

	"github.com/andreimladin/dongle/internal/builtins"
)

// updateCmd replaces the old `dongle refresh` (and before it `dongle index
// refresh`): checks the feed for the latest index version, ignoring the
// TTL cache, and downloads the index only when that version is newer.
var updateCmd = &cobra.Command{
	Use:     "update",
	Short:   "Refresh the plugin index from the feed",
	Long:    "Check the feed for a newer plugin index, download it if there is one, and show its version and plugins.",
	Example: `  dongle update`,
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return exitCode(builtins.Update())
	},
}
