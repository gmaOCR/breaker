// Package core holds shared domain types with no logic, to avoid import cycles.
package core

import "time"

// Provider identifies which upstream LLM API a request targets.
type Provider string

const (
	ProviderAnthropic Provider = "anthropic"
	ProviderOpenAI    Provider = "openai"
)

// SessionID groups spend under one budget: one run, or one key/project in serve mode.
type SessionID string

// Usage is the token accounting for a single LLM response.
type Usage struct {
	InputTokens      int
	OutputTokens     int
	CacheWriteTokens int
	CacheReadTokens  int
}

// SpendEvent is one metered LLM response with its computed cost.
type SpendEvent struct {
	Session   SessionID
	Provider  Provider
	Model     string
	Usage     Usage
	CostUSD   float64
	Estimated bool   // true when cost/tokens were estimated, not provider-reported
	ReqHash   string // fingerprint of the request body, for loop/dedup detection
	At        time.Time
}

// TripReason explains why the breaker fired.
type TripReason struct {
	Policy    string
	Message   string
	SpentUSD  float64
	BudgetUSD float64
}
