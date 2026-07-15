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

func usage() {
	fmt.Println(`breaker — cost circuit-breaker for AI agents

usage:
  breaker run [flags] -- <command> [args...]   guard a single agent run with a hard cap
  breaker serve [flags]                        standalone proxy + dashboard (rolling budget)
  breaker version                              print version and pricing-table date

run flags:
  --budget <usd>     hard USD cap, e.g. --budget 5.00
  --tokens <n>       hard token cap
  --grace <dur>      SIGTERM→SIGKILL grace period (default 3s)
  --anthropic-upstream <url>   override Anthropic upstream (default https://api.anthropic.com)
  --openai-upstream <url>      override OpenAI upstream (default https://api.openai.com)
  --prices <file>    pricing override JSON

example:
  breaker run --budget 0.50 -- claude -p "refactor this repo"

docs: https://github.com/gmaOCR/breaker`)
}
