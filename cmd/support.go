package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/andreimladin/dongle/internal/index"
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
		return exitCode(support(args[0]))
	},
}

func support(name string) int {
	if err := index.EnsureFresh(plugincmd.IndexTTL); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}

	m, err := index.Load(name)
	if errors.Is(err, index.ErrNotFound) {
		fmt.Fprintf(os.Stderr, "error: no plugin named %s in the index\n", name)
		return 1
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}

	fmt.Printf("%s — support\n", m.Name)
	fmt.Printf("  Documentation: %s\n", m.Support.Documentation)
	fmt.Printf("  Channel:       %s\n", m.Support.Channel)
	if m.Support.Contact != "" {
		fmt.Printf("  Contact:       %s\n", m.Support.Contact)
	}
	return 0
}
