package main

import (
	"github.com/spf13/cobra"

	"github.com/andreimladin/dongle/internal/builtins"
)

// updateCmd replaces the old `dongle refresh` (and before it `dongle index
// refresh`): force-downloads the latest index from the feed, ignoring the
// TTL cache.
var updateCmd = &cobra.Command{
	Use:     "update",
	Short:   "Refresh the plugin index from the feed",
	Long:    "Download the latest plugin index and show its version and plugins.",
	Example: `  dongle update`,
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return exitCode(builtins.Update())
	},
}
