package main

import (
	"github.com/spf13/cobra"

	"github.com/andreimladin/dongle/internal/hostcmd"
)

var refreshCmd = &cobra.Command{
	Use:   "refresh",
	Short: "force-download the latest plugin index from the feed",
	Long:  "Force-download the latest plugin index from the feed, ignoring the cache's TTL.",
	Run:   func(cmd *cobra.Command, args []string) { exitCode(hostcmd.Refresh(args)) },
}
