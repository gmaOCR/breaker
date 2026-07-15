package policy

import (
	"testing"
	"time"

	"github.com/gmaOCR/breaker/internal/core"
)

var base = time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)

func at(sec int, cost float64) core.SpendEvent {
	return core.SpendEvent{At: base.Add(time.Duration(sec) * time.Second), CostUSD: cost}
}

func TestVelocityCallsPerMin(t *testing.T) {
	v := NewVelocity(0, 3) // max 3 calls/minute
	for i, sec := range []int{0, 1, 2} {
		if trip, _ := v.Check(State{}, at(sec, 0.001)); trip {
			t.Fatalf("tripped at call %d (still within 3/min)", i+1)
		}
	}
	if trip, _ := v.Check(State{}, at(3, 0.001)); !trip {
		t.Fatal("did not trip on the 4th call within a minute")
	}
}

func TestVelocityUSDPerMin(t *testing.T) {
	v := NewVelocity(0.05, 0) // max $0.05/minute
	if trip, _ := v.Check(State{}, at(0, 0.02)); trip {
		t.Fatal("tripped at $0.02")
	}
	if trip, _ := v.Check(State{}, at(1, 0.02)); trip {
		t.Fatal("tripped at $0.04")
	}
	if trip, _ := v.Check(State{}, at(2, 0.02)); !trip {
		t.Fatal("did not trip at $0.06 over $0.05/min")
	}
}

func TestVelocityPrunesOldEvents(t *testing.T) {
	v := NewVelocity(0, 3)
	for _, sec := range []int{0, 1, 2} {
		v.Check(State{}, at(sec, 0.001))
	}
	// 70s later, the earlier three have aged out of the 60s window.
	if trip, _ := v.Check(State{}, at(70, 0.001)); trip {
		t.Fatal("tripped after old events should have been pruned")
	}
}
