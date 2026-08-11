// Command dongle is the unified host CLI. Builtins are cobra commands
// (see root.go, plugin.go, index.go); anything else is resolved to an
// installed plugin and exec'd via internal/dispatch (see root.go's
// fall-through RunE).
package main

import "os"

func main() { os.Exit(Execute()) }
