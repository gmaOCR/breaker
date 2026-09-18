package main

import (
	"fmt"

	"github.com/gmaOCR/breaker/internal/pricing"
)

// version is overridden at release time via -ldflags "-X main.version=...".
var version = "0.1.0-dev"

func cmdVersion() {
	pv := "unknown"
	if t, err := pricing.Load(""); err == nil {
		pv = t.Version
	}
	fmt.Printf("breaker %s (pricing table %s)\n", version, pv)
}

// usage lists the subcommands only. The per-subcommand flag lists come from
// each FlagSet's own PrintDefaults, so `breaker run -h` cannot drift from the
// flags that actually exist; a second copy here always did.
func usage() {
	fmt.Println(`breaker: cost circuit-breaker for AI agents

usage:
  breaker run [flags] -- <command> [args...]   guard a single agent run with a hard cap
  breaker serve [flags]                        standalone proxy + dashboard (rolling budget)
  breaker version                              print version and pricing-table date

flags:
  breaker run -h      budget, token and velocity caps, grace period, upstreams, notifications
  breaker serve -h    rolling window, journal, dashboard port, upstreams, notifications

example:
  breaker run --budget 0.50 -- claude -p "refactor this repo"

docs: https://github.com/gmaOCR/breaker`)
}
