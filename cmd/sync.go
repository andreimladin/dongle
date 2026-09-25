package main

import (
	"github.com/spf13/cobra"

	"github.com/andreimladin/dongle/internal/builtins"
)

// syncCmd replaces the old `dongle refresh` (and before it `dongle index
// refresh`): force-downloads the latest index from the feed, ignoring the
// TTL cache.
var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Download the latest plugin index from the feed",
	Long: `Download the latest version of the plugin index from the feed into the
local cache, ignoring the cache TTL, and show what it contains.

If the feed can't be reached, the existing cache (or the index embedded in
this binary) stays in use.`,
	Example: `  dongle sync`,
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return exitCode(builtins.Sync())
	},
}
