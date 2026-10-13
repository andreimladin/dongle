package builtins

import "github.com/andreimladin/dongle/internal/index"

// Update refreshes the plugin index from the feed, ignoring the TTL
// (`dongle update`); see index.Update.
func Update() int {
	if err := index.Update(); err != nil {
		return 1
	}
	return 0
}
