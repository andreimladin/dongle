package main

import (
	"github.com/spf13/cobra"

	"github.com/andreimladin/dongle/internal/hostcmd"
)

var supportCmd = &cobra.Command{
	Use:   "support <plugin-name>",
	Short: "show where to get help with a plugin",
	Long:  "Show a plugin's documentation, support channel and contact, read from the index (the plugin need not be installed).",
	Run:   func(cmd *cobra.Command, args []string) { exitCode(hostcmd.Support(args)) },
}
