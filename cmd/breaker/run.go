package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/gmaOCR/breaker/internal/breaker"
	"github.com/gmaOCR/breaker/internal/core"
	"github.com/gmaOCR/breaker/internal/policy"
	"github.com/gmaOCR/breaker/internal/pricing"
	"github.com/gmaOCR/breaker/internal/proxy"
	"github.com/gmaOCR/breaker/internal/runner"
)

func cmdRun(args []string) int {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	budget := fs.Float64("budget", 0, "hard USD spend cap for this run (0 = no USD cap)")
	tokens := fs.Int("tokens", 0, "hard token cap for this run (0 = no token cap)")
	grace := fs.Duration("grace", 3*time.Second, "grace period between SIGTERM and SIGKILL")
	port := fs.Int("port", 0, "proxy port (0 = random free port)")
	pricesF := fs.String("prices", "", "path to a pricing override JSON file")
	anthUp := fs.String("anthropic-upstream", "", "override Anthropic upstream base URL")
	oaiUp := fs.String("openai-upstream", "", "override OpenAI upstream base URL")
	maxPerMin := fs.Float64("max-per-min", 0, "velocity guard: trip if spend exceeds this USD/minute (0 = off)")
	maxCalls := fs.Int("max-calls-per-min", 0, "velocity guard: trip if calls exceed this per minute (0 = off)")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: breaker run [flags] -- <command> [args...]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	argv := fs.Args()
	if len(argv) == 0 {
		fmt.Fprintln(os.Stderr, "breaker run: missing command (use: breaker run --budget N -- <command>)")
		return 2
	}
	if *budget <= 0 && *tokens <= 0 {
		fmt.Fprintln(os.Stderr, "breaker run: set at least one of --budget or --tokens")
		return 2
	}

	prices, err := pricing.Load(*pricesF)
	if err != nil {
		fmt.Fprintf(os.Stderr, "breaker: %v\n", err)
		return 1
	}
	pols := []policy.Policy{policy.HardCap{}}
	if *maxPerMin > 0 || *maxCalls > 0 {
		pols = append(pols, policy.NewVelocity(*maxPerMin, *maxCalls))
	}
	engine := breaker.New(breaker.Config{BudgetUSD: *budget, TokenBudget: *tokens, Policies: pols})
	session := newSessionID()
	pxy, err := proxy.New(engine, prices, proxy.Config{
		Session:           session,
		AnthropicUpstream: *anthUp,
		OpenAIUpstream:    *oaiUp,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "breaker: %v\n", err)
		return 1
	}

	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *port))
	if err != nil {
		fmt.Fprintf(os.Stderr, "breaker: listen: %v\n", err)
		return 1
	}
	srv := &http.Server{Handler: pxy}
	go func() { _ = srv.Serve(ln) }()
	defer func() { _ = srv.Close() }()

	base := "http://" + ln.Addr().String()
	env := append(os.Environ(),
		"ANTHROPIC_BASE_URL="+base,
		"OPENAI_BASE_URL="+base+"/v1",
		"OPENAI_API_BASE="+base+"/v1",
		"BREAKER_SESSION="+string(session),
	)

	fmt.Fprintf(os.Stderr, "breaker: guarding run (cap %s) — proxy at %s\n", capLabel(*budget, *tokens), base)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	code, _, err := runner.Run(ctx, argv, env, engine.Trips(), *grace)
	if err != nil {
		fmt.Fprintf(os.Stderr, "breaker: run error: %v\n", err)
		return 1
	}

	snap := engine.Snapshot()
	est := ""
	if snap.Estimated {
		est = " (estimated — some usage was not provider-reported)"
	}
	fmt.Fprintf(os.Stderr, "breaker: spent $%.4f over %d tokens%s\n", snap.SpentUSD, snap.Tokens, est)
	if snap.Tripped {
		fmt.Fprintf(os.Stderr, "breaker: TRIPPED — %s [%s]\n", snap.Reason.Message, snap.Reason.Policy)
		if code == 0 {
			code = runner.TripExitCode // child slipped out via 402 before the kill landed
		}
	}
	return code
}

func capLabel(budget float64, tokens int) string {
	s := ""
	if budget > 0 {
		s = fmt.Sprintf("$%.2f", budget)
	}
	if tokens > 0 {
		if s != "" {
			s += " / "
		}
		s += fmt.Sprintf("%d tok", tokens)
	}
	return s
}

func newSessionID() core.SessionID {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return core.SessionID(hex.EncodeToString(b))
}
