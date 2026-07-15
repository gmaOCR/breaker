package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/gmaOCR/breaker/internal/core"
	"github.com/gmaOCR/breaker/internal/dashboard"
	"github.com/gmaOCR/breaker/internal/pricing"
	"github.com/gmaOCR/breaker/internal/proxy"
	"github.com/gmaOCR/breaker/internal/store"
)

// cmdServe runs the standalone proxy + dashboard with a rolling-window budget
// (use case 3): a shared cap across all runs on a key, for CI / cron / fleets.
func cmdServe(args []string) int {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	daily := fs.Float64("daily", 0, "rolling 24h USD budget (0 = off)")
	hourly := fs.Float64("hourly", 0, "rolling 1h USD budget (0 = off)")
	port := fs.Int("port", 8900, "listen port (proxy + dashboard share it)")
	journal := fs.String("journal", "", "JSONL spend journal path (persists the rolling window across restarts)")
	pricesF := fs.String("prices", "", "pricing override JSON")
	anthUp := fs.String("anthropic-upstream", "", "override Anthropic upstream base URL")
	oaiUp := fs.String("openai-upstream", "", "override OpenAI upstream base URL")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: breaker serve [--daily N | --hourly N] [flags]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}

	var (
		window time.Duration
		budget float64
		label  string
	)
	switch {
	case *hourly > 0:
		window, budget, label = time.Hour, *hourly, "hourly"
	case *daily > 0:
		window, budget, label = 24*time.Hour, *daily, "daily"
	default:
		fmt.Fprintln(os.Stderr, "breaker serve: set --daily or --hourly")
		return 2
	}

	prices, err := pricing.Load(*pricesF)
	if err != nil {
		fmt.Fprintf(os.Stderr, "breaker: %v\n", err)
		return 1
	}
	st, err := store.Open(*journal, window)
	if err != nil {
		fmt.Fprintf(os.Stderr, "breaker: journal: %v\n", err)
		return 1
	}
	defer func() { _ = st.Close() }()

	guard := &windowGuard{store: st, budgetUSD: budget, window: window, label: label}
	// Empty Session → the proxy attributes each request to a session itself.
	pxy, err := proxy.New(guard, prices, proxy.Config{AnthropicUpstream: *anthUp, OpenAIUpstream: *oaiUp})
	if err != nil {
		fmt.Fprintf(os.Stderr, "breaker: %v\n", err)
		return 1
	}

	mux := http.NewServeMux()
	mux.Handle("/v1/", pxy)                   // LLM traffic
	mux.Handle("/", dashboard.Handler(guard)) // UI + /api/state + /kill

	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *port))
	if err != nil {
		fmt.Fprintf(os.Stderr, "breaker: listen: %v\n", err)
		return 1
	}
	base := "http://" + ln.Addr().String()
	fmt.Fprintf(os.Stderr, "breaker: serving (%s budget $%.2f) — dashboard %s\n", label, budget, base+"/")
	fmt.Fprintf(os.Stderr, "point your agent at it: export ANTHROPIC_BASE_URL=%s\n", base)
	if err := http.Serve(ln, mux); err != nil {
		fmt.Fprintf(os.Stderr, "breaker: %v\n", err)
		return 1
	}
	return 0
}

// windowGuard enforces a rolling-window budget plus a manual kill. It satisfies
// both proxy.Guard and dashboard.Controller.
type windowGuard struct {
	store     *store.Store
	budgetUSD float64
	window    time.Duration
	label     string

	mu      sync.Mutex
	killed  bool
	killMsg string
}

func (g *windowGuard) Allowed() (bool, core.TripReason) {
	g.mu.Lock()
	killed, msg := g.killed, g.killMsg
	g.mu.Unlock()
	if killed {
		return false, core.TripReason{Policy: "manual", Message: msg, BudgetUSD: g.budgetUSD}
	}
	spent := g.store.WindowSum(g.window)
	if g.budgetUSD > 0 && spent >= g.budgetUSD {
		return false, core.TripReason{
			Policy:    "window",
			Message:   fmt.Sprintf("%s budget $%.2f reached ($%.4f spent)", g.label, g.budgetUSD, spent),
			SpentUSD:  spent,
			BudgetUSD: g.budgetUSD,
		}
	}
	return true, core.TripReason{}
}

func (g *windowGuard) Record(ev core.SpendEvent) (bool, core.TripReason) {
	g.store.Add(store.Event{
		At: ev.At, Session: ev.Session, Model: ev.Model,
		CostUSD: ev.CostUSD, Estimated: ev.Estimated,
	})
	return false, core.TripReason{}
}

func (g *windowGuard) Kill(reason string) {
	g.mu.Lock()
	g.killed, g.killMsg = true, reason
	g.mu.Unlock()
}

func (g *windowGuard) State() dashboard.State {
	g.mu.Lock()
	killed := g.killed
	g.mu.Unlock()

	sums := g.store.SessionSums(g.window)
	sessions := make([]dashboard.SessionStat, 0, len(sums))
	for sid, usd := range sums {
		sessions = append(sessions, dashboard.SessionStat{Session: string(sid), USD: usd})
	}
	sort.Slice(sessions, func(i, j int) bool { return sessions[i].USD > sessions[j].USD })

	recent := g.store.Recent(20)
	rev := make([]dashboard.RecentEvent, 0, len(recent))
	for _, e := range recent {
		rev = append(rev, dashboard.RecentEvent{
			At: e.At.Format("15:04:05"), Session: string(e.Session),
			Model: e.Model, USD: e.CostUSD, Estimated: e.Estimated,
		})
	}
	return dashboard.State{
		WindowLabel: g.label,
		BudgetUSD:   g.budgetUSD,
		SpentUSD:    g.store.WindowSum(g.window),
		Killed:      killed,
		Sessions:    sessions,
		Recent:      rev,
	}
}
