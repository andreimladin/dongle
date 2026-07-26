// Command dongle-deploy is a sample dongle plugin built with cobra. Invoked as
// `dongle deploy ...`; cobra routes its own subcommands and flags unchanged.
//
// It demonstrates the key point: an existing cobra CLI becomes a dongle plugin
// by setting the root command's Use to the plugin name and shipping a
// plugin.json. It needs nothing from dongle — the host's context arrives as
// plain environment variables, read here with os.Getenv.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	hostVersion := os.Getenv("DONGLE_VERSION") // injected by the host

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
			fmt.Printf("deploying %q (host %s)\n", args[0], hostVersion)
			if wait {
				fmt.Println("  waiting for rollout... done")
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
