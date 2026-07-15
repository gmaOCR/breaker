// Command breaker is a cost circuit-breaker for AI agents: it enforces a hard
// spending cap on an agent run and kills it before a runaway bill.
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "run":
		os.Exit(cmdRun(os.Args[2:]))
	case "serve":
		os.Exit(cmdServe(os.Args[2:]))
	case "version", "-v", "--version":
		cmdVersion()
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "breaker: unknown command %q\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}
