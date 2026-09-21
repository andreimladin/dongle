package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/andreimladin/dongle/internal/index"
)

// refreshCmd replaces the old `dongle index refresh`: force-downloads the
// latest index archive from the feed, ignoring the TTL cache.
var refreshCmd = &cobra.Command{
	Use:   "refresh",
	Short: "force-download the latest plugin index from the feed",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := index.Refresh(); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return exitCode(1)
		}
		v, _ := index.CachedVersion()
		fmt.Printf("index refreshed (%s)\n", v)
		return nil
	},
}
