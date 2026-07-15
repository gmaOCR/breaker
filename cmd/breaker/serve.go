package main

import (
	"fmt"
	"os"
)

// cmdServe will host the standalone proxy + dashboard with a rolling-window
// budget (use case 3). Implemented in a later build phase.
func cmdServe(_ []string) int {
	fmt.Fprintln(os.Stderr, "breaker serve: not implemented yet — coming in a later phase")
	return 1
}
