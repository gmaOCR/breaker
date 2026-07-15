// Package breaker is the budget engine: it accumulates spend, evaluates policies,
// and fires a one-shot trip that the runner and dashboard react to.
package breaker

import (
	"sync"

	"github.com/gmaOCR/breaker/internal/core"
	"github.com/gmaOCR/breaker/internal/policy"
)

// Config configures a new Engine.
type Config struct {
	BudgetUSD   float64
	TokenBudget int
	Policies    []policy.Policy // defaults to HardCap when empty
}

// Engine is safe for concurrent use by the proxy (many in-flight responses).
type Engine struct {
	budgetUSD   float64
	tokenBudget int
	policies    []policy.Policy

	mu        sync.Mutex
	spentUSD  float64
	tokens    int
	estimated bool
	tripped   bool
	reason    core.TripReason

	trips chan core.TripReason
}

// New builds an Engine. With no policies it uses a single HardCap.
func New(cfg Config) *Engine {
	pols := cfg.Policies
	if len(pols) == 0 {
		pols = []policy.Policy{policy.HardCap{}}
	}
	return &Engine{
		budgetUSD:   cfg.BudgetUSD,
		tokenBudget: cfg.TokenBudget,
		policies:    pols,
		trips:       make(chan core.TripReason, 1),
	}
}

// Record adds a spend event and evaluates policies. It returns true (with the
// reason) when this event caused the breaker to trip. Trips are one-shot.
func (e *Engine) Record(ev core.SpendEvent) (bool, core.TripReason) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.spentUSD += ev.CostUSD
	e.tokens += ev.Usage.InputTokens + ev.Usage.OutputTokens
	if ev.Estimated {
		e.estimated = true
	}
	if e.tripped {
		return false, e.reason
	}
	st := policy.State{
		SpentUSD:    e.spentUSD,
		Tokens:      e.tokens,
		BudgetUSD:   e.budgetUSD,
		TokenBudget: e.tokenBudget,
	}
	for _, p := range e.policies {
		if trip, msg := p.Check(st, ev); trip {
			return e.tripLocked(core.TripReason{
				Policy:    p.Name(),
				Message:   msg,
				SpentUSD:  e.spentUSD,
				BudgetUSD: e.budgetUSD,
			})
		}
	}
	return false, core.TripReason{}
}

// Kill forces a manual trip (e.g. the dashboard KILL button).
func (e *Engine) Kill(message string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.tripped {
		return
	}
	e.tripLocked(core.TripReason{
		Policy:    "manual",
		Message:   message,
		SpentUSD:  e.spentUSD,
		BudgetUSD: e.budgetUSD,
	})
}

// tripLocked must be called with e.mu held.
func (e *Engine) tripLocked(r core.TripReason) (bool, core.TripReason) {
	e.tripped = true
	e.reason = r
	select {
	case e.trips <- r:
	default:
	}
	return true, r
}

// Tripped reports whether the breaker has fired.
func (e *Engine) Tripped() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.tripped
}

// Reason returns the trip reason (zero value if not tripped).
func (e *Engine) Reason() core.TripReason {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.reason
}

// Trips delivers the trip reason exactly once.
func (e *Engine) Trips() <-chan core.TripReason { return e.trips }

// Snapshot is a read-only view for the run summary and the dashboard.
type Snapshot struct {
	SpentUSD  float64
	Tokens    int
	BudgetUSD float64
	Estimated bool
	Tripped   bool
	Reason    core.TripReason
}

// Snapshot returns the current state atomically.
func (e *Engine) Snapshot() Snapshot {
	e.mu.Lock()
	defer e.mu.Unlock()
	return Snapshot{
		SpentUSD:  e.spentUSD,
		Tokens:    e.tokens,
		BudgetUSD: e.budgetUSD,
		Estimated: e.estimated,
		Tripped:   e.tripped,
		Reason:    e.reason,
	}
}
