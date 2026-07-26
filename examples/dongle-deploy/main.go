// Command dongle-deploy is a sample dongle plugin built with cobra. Invoked as
// `dongle deploy ...`; cobra routes its own subcommands and flags unchanged.
//
// This demonstrates the key point: an existing cobra CLI becomes a dongle plugin
// by setting the root command's Use to the plugin name and shipping a
// plugin.json — its whole subcommand tree keeps working, because dongle
// dispatches by process and hands the args straight to cobra.
package main

import (
	"fmt"
	"os"

	sdk "github.com/andreimladin/dongle/pkg/pluginsdk"
	"github.com/spf13/cobra"
)

func main() {
	ctx := sdk.LoadContext()

	root := &cobra.Command{
		Use:          "deploy", // matches manifest name -> `dongle deploy`
		Short:        "Deploy services to the platform",
		SilenceUsage: true,
	}

	root.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Show service health",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("all services healthy")
			return nil
		},
	})

	var wait bool
	runCmd := &cobra.Command{
		Use:   "run <service>",
		Short: "Deploy a service",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := args[0]

			// Consume the token the host brokered — no login flow here.
			tok, ok := sdk.Token("platform-oidc")
			if !ok {
				return fmt.Errorf("no platform-oidc credential brokered (run: dongle login platform-oidc)")
			}

			fmt.Printf("deploying %q (host %s, invocation %s)\n", svc, ctx.HostVersion, ctx.InvocationID)
			fmt.Printf("  auth: platform-oidc token %s\n", redact(tok))
			if wait {
				fmt.Println("  waiting for rollout… done")
			}
			return nil
		},
	}
	runCmd.Flags().BoolVar(&wait, "wait", false, "wait for rollout to finish")
	root.AddCommand(runCmd)

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func redact(s string) string {
	if len(s) <= 8 {
		return "****"
	}
	return s[:8] + "…"
}
