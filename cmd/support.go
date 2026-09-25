package main

import (
	"github.com/spf13/cobra"

	"github.com/andreimladin/dongle/internal/plugincmd"
)

// supportCmd reads a plugin's support links straight from the index
// manifest — it never consults installed state, so it works for any plugin
// in the index whether or not it's installed locally.
var supportCmd = &cobra.Command{
	Use:   "support <plugin-name>",
	Short: "Show where to get help with a plugin",
	Long: `Show a plugin's documentation, support channel and contact, as declared
in its index manifest. Works for any plugin in the index, installed or not.`,
	Example: `  dongle support deploy`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return exitCode(plugincmd.Support(args[0]))
	},
}
