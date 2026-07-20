// Command dongle is the unified host CLI. Builtins are handled in-process;
// anything else is resolved to an installed plugin and exec'd.
package main

import (
	"fmt"
	"os"

	"github.com/andreimladin/dongle/internal/auth"
	"github.com/andreimladin/dongle/internal/dispatch"
	"github.com/andreimladin/dongle/internal/plugincmd"
)

const (
	hostVersion = "2.5.0" // the host CLI's own semver
	protocol    = "v1"    // plugin protocol version this host speaks
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
		return plugincmd.Run(args[1:])

	default:
		// Unknown builtin → treat as a plugin command name.
		return dispatch.Run(hostVersion, protocol, args[0], args[1:])
	}
}

// usage is hand-rolled for a zero-dependency scaffold. Swap this whole builtin
// switch for spf13/cobra when you want grouped help, flag parsing, and shell
// completion — the dispatch.Run fallthrough becomes cobra's
// Command.RunE / the unknown-command hook.
func usage() {
	fmt.Print(`dongle — unified CLI

Builtins:
  dongle version
  dongle login <provider>          obtain + store a credential
  dongle logout <provider>
  dongle plugin list
  dongle plugin install <dir>      install a plugin from a local build dir
  dongle plugin uninstall <name>
  dongle <name> [args...]          run an installed plugin

`)
}
