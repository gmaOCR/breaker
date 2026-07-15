package pricing

import (
	"testing"

	"github.com/gmaOCR/breaker/internal/core"
)

func TestCost(t *testing.T) {
	tbl, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	oneM := core.Usage{InputTokens: 1_000_000, OutputTokens: 1_000_000}

	cases := []struct {
		model     string
		want      float64
		wantMatch bool
	}{
		{"claude-opus-4-8", 30, true},          // 5 + 25
		{"claude-opus-4-5-20251101", 30, true}, // dated snapshot, glob match
		{"claude-sonnet-5", 18, true},          // 3 + 15
		{"claude-haiku-4-5", 6, true},          // 1 + 5
		{"claude-fable-5", 60, true},           // 10 + 50
		{"gpt-4o", 12.5, true},                 // 2.5 + 10
		{"totally-unknown-model", 100, false},  // fallback 20 + 80, NOT matched
	}
	for _, c := range cases {
		got, matched := tbl.Cost(c.model, oneM)
		if got != c.want || matched != c.wantMatch {
			t.Errorf("Cost(%q) = %v,%v; want %v,%v", c.model, got, matched, c.want, c.wantMatch)
		}
	}
}

func TestCostNeverZeroForUsage(t *testing.T) {
	tbl, _ := Load("")
	// The cardinal bug: an unknown model must never price to zero, or the
	// breaker would never trip.
	got, matched := tbl.Cost("some-future-model-2027", core.Usage{InputTokens: 100_000})
	if got <= 0 {
		t.Fatalf("unknown model priced to %v (must be > 0)", got)
	}
	if matched {
		t.Fatalf("unknown model should report matched=false")
	}
}
