// Package policy holds the pluggable checks the breaker engine evaluates after
// each metered spend event.
package policy

import "github.com/gmaOCR/breaker/internal/core"

// State is the read-only view a policy needs to decide whether to trip.
type State struct {
	SpentUSD    float64
	Tokens      int
	BudgetUSD   float64
	TokenBudget int
}

// Policy decides whether the breaker should trip given current state and the
// event that just landed.
type Policy interface {
	Name() string
	Check(s State, ev core.SpendEvent) (trip bool, message string)
}
