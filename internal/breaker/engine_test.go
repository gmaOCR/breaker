package breaker

import (
	"testing"

	"github.com/gmaOCR/breaker/internal/core"
)

func ev(cost float64) core.SpendEvent {
	return core.SpendEvent{Model: "test", Usage: core.Usage{OutputTokens: 100}, CostUSD: cost}
}

func TestHardCapTripsOnce(t *testing.T) {
	e := New(Config{BudgetUSD: 0.05})

	if trip, _ := e.Record(ev(0.02)); trip {
		t.Fatal("tripped at $0.02 under a $0.05 budget")
	}
	if trip, _ := e.Record(ev(0.02)); trip {
		t.Fatal("tripped at $0.04 under a $0.05 budget")
	}
	trip, reason := e.Record(ev(0.02)) // now $0.06 >= $0.05
	if !trip {
		t.Fatal("did not trip at $0.06 over a $0.05 budget")
	}
	if reason.Policy != "hardcap" {
		t.Errorf("policy = %q; want hardcap", reason.Policy)
	}

	// The trip fires exactly once on the channel.
	select {
	case <-e.Trips():
	default:
		t.Fatal("trip channel was empty")
	}

	// A later event must not re-trip (one-shot).
	if trip, _ := e.Record(ev(1.0)); trip {
		t.Fatal("re-tripped after already tripped")
	}
}

func TestTokenCap(t *testing.T) {
	e := New(Config{TokenBudget: 150})
	// each ev() adds 100 output tokens
	if trip, _ := e.Record(ev(0)); trip {
		t.Fatal("tripped at 100 tokens under a 150 budget")
	}
	if trip, _ := e.Record(ev(0)); !trip {
		t.Fatal("did not trip at 200 tokens over a 150 budget")
	}
}
