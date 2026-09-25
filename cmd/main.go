// Command dongle is the unified host CLI. Builtins are cobra commands
// (see root.go, plugins.go, sync.go, support.go); anything else is resolved to an
// installed plugin and exec'd via internal/dispatch (see root.go's
// fall-through RunE).
package main

import "os"

func main() { os.Exit(Execute()) }
