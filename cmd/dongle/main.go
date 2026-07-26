// Command dongle is the unified host CLI. Builtins are handled in-process;
// anything else is resolved to an installed plugin and exec'd.
//
// The hand-rolled builtin switch is intentional for a zero-dependency host core.
// To grow it up, swap this switch for spf13/cobra: builtins become cobra
// commands, and the default (plugin) case becomes cobra's unknown-command hook
// calling dispatch.Run. The plugin dispatch logic itself does not change.
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/andreimladin/dongle/internal/auth"
	"github.com/andreimladin/dongle/internal/dispatch"
	"github.com/andreimladin/dongle/internal/index"
	"github.com/andreimladin/dongle/internal/plugincmd"
)

const (
	hostVersion = "0.1.0" // the host's own semver
	protocol    = "v1"    // the host<->plugin contract version
)

func main() { os.Exit(run(os.Args[1:])) }

func run(args []string) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		usage()
		return 0
	}

	switch args[0] {
	case "version":
		fmt.Printf("dongle %s (protocol %s)\n", hostVersion, protocol)
		return 0

	case "login":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: dongle login <provider>")
			return 2
		}
		if err := auth.Login(args[1]); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return 1
		}
		fmt.Printf("logged in to %s\n", args[1])
		return 0

	case "logout":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: dongle logout <provider>")
			return 2
		}
		if err := auth.Logout(args[1]); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return 1
		}
		fmt.Printf("logged out of %s\n", args[1])
		return 0

	case "plugin":
		return plugincmd.Run(hostVersion, protocol, args[1:])

	case "index":
		return indexCmd(args[1:])

	default:
		// Not a builtin -> resolve to an installed plugin and exec it.
		return dispatch.Run(hostVersion, protocol, args[0], args[1:])
	}
}

func indexCmd(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: dongle index <refresh|status>")
		return 2
	}
	switch args[0] {
	case "refresh":
		if err := index.Refresh(); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return 1
		}
		fmt.Println("index refreshed")
		return 0
	case "status":
		url, age, cloned := index.Status()
		if !cloned {
			fmt.Printf("index: %s (not yet cloned)\n", url)
		} else {
			fmt.Printf("index: %s\ncache age: %s\n", url, age.Round(time.Second))
		}
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown index subcommand %q\n", args[0])
		return 2
	}
}

func usage() {
	fmt.Print(`dongle — one CLI, plug in the rest

Builtins:
  dongle version
  dongle login <provider>          obtain + store a credential
  dongle logout <provider>
  dongle plugin search             list plugins available in the index
  dongle plugin install <name>     install a plugin from the index
  dongle plugin install <dir>      install a plugin from a local build dir
  dongle plugin list               list installed plugins
  dongle plugin uninstall <name>
  dongle index refresh             force-refresh the index cache
  dongle index status
  dongle <name> [args...]          run an installed plugin

`)
}
