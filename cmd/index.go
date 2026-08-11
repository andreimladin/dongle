package main

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/andreimladin/dongle/internal/index"
)

// indexCmd's own RunE only fires when no subcommand (or an unrecognized
// one) was given — refresh/status below are matched and handled directly
// by cobra. Mirrors the old hand-rolled indexCmd switch's usage text and
// exit codes exactly.
var indexCmd = &cobra.Command{
	Use:   "index",
	Short: "manage the plugin index cache",
	Args:  cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			fmt.Fprintln(os.Stderr, "usage: dongle index <refresh|status>")
			return exitCode(2)
		}
		fmt.Fprintf(os.Stderr, "unknown index subcommand %q\n", args[0])
		return exitCode(2)
	},
}

var indexRefreshCmd = &cobra.Command{
	Use:   "refresh",
	Short: "force-refresh the index cache",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := index.Refresh(); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return exitCode(1)
		}
		fmt.Println("index refreshed")
		return nil
	},
}

var indexStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "show index cache status",
	RunE: func(cmd *cobra.Command, args []string) error {
		url, branch, age, cloned := index.Status()
		if !cloned {
			fmt.Printf("index: %s (branch %s) (not yet cloned)\n", url, branch)
		} else {
			fmt.Printf("index: %s (branch %s)\ncache age: %s\n", url, branch, age.Round(time.Second))
		}
		return nil
	},
}

func init() {
	indexCmd.AddCommand(indexRefreshCmd, indexStatusCmd)
}
