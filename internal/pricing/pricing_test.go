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
		{"claude-opus-5", 30, true},            // 5 + 25
		{"claude-opus-4-8", 30, true},          // 5 + 25
		{"claude-opus-4-5-20251101", 30, true}, // dated snapshot, glob match
		{"claude-sonnet-5", 12, true},          // 2 + 10
		{"claude-sonnet-4-6", 18, true},        // 3 + 15
		{"claude-haiku-4-5", 6, true},          // 1 + 5
		{"claude-fable-5", 60, true},           // 10 + 50
		{"claude-fable-5-1", 60, true},         // 10 + 50, differs only on cache reads
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

// TestCurrentModelsAreMatched guards the bug this table is most prone to: a new
// model ships, no pattern covers its id, and every run on it silently prices at
// the fallback. That never under-bills, but it trips the breaker early and
// reports a spend that is plainly wrong. Add each model you actually run here.
func TestCurrentModelsAreMatched(t *testing.T) {
	tbl, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	current := []string{
		"claude-opus-5",
		"claude-opus-4-8",
		"claude-sonnet-5",
		"claude-fable-5-1",
		"claude-haiku-4-5",
		"gpt-4o",
		"gpt-4.1",
	}
	for _, m := range current {
		if _, matched := tbl.Cost(m, core.Usage{InputTokens: 1000}); !matched {
			t.Errorf("%q fell through to the fallback price; add a pattern for it", m)
		}
	}
}

// TestCacheReadRatesDiffer pins the one place two current models share an input
// price but not a cache-read price: Fable 5.1 reads cache at 0.025x base, every
// other model at 0.1x. The exact key must win over the claude-fable-5* glob.
func TestCacheReadRatesDiffer(t *testing.T) {
	tbl, _ := Load("")
	oneM := core.Usage{CacheReadTokens: 1_000_000}

	got51, _ := tbl.Cost("claude-fable-5-1", oneM)
	got5, _ := tbl.Cost("claude-fable-5", oneM)
	if got51 != 0.25 {
		t.Errorf("claude-fable-5-1 cache read = %v; want 0.25", got51)
	}
	if got5 != 1 {
		t.Errorf("claude-fable-5 cache read = %v; want 1", got5)
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
