package policy

import (
	"fmt"
	"sync"
	"time"

	"github.com/gmaOCR/breaker/internal/core"
)

// Velocity trips EARLY — before the absolute budget cap — when spend or call
// rate over a rolling one-minute window spikes, catching runaway loops before
// they run up the full budget. A zeroed threshold disables that check.
type Velocity struct {
	MaxUSDPerMin   float64
	MaxCallsPerMin int

	window time.Duration
	mu     sync.Mutex
	events []velEvent
}

type velEvent struct {
	at   time.Time
	cost float64
}

// NewVelocity builds a velocity guard. window is fixed at one minute.
func NewVelocity(maxUSDPerMin float64, maxCallsPerMin int) *Velocity {
	return &Velocity{MaxUSDPerMin: maxUSDPerMin, MaxCallsPerMin: maxCallsPerMin, window: time.Minute}
}

func (v *Velocity) Name() string { return "velocity" }

func (v *Velocity) Check(_ State, ev core.SpendEvent) (bool, string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	now := ev.At
	if now.IsZero() {
		now = time.Now()
	}
	v.events = append(v.events, velEvent{now, ev.CostUSD})

	cut := now.Add(-v.window)
	drop := 0
	for drop < len(v.events) && v.events[drop].at.Before(cut) {
		drop++
	}
	v.events = v.events[drop:]

	var sum float64
	for _, e := range v.events {
		sum += e.cost
	}
	if v.MaxUSDPerMin > 0 && sum > v.MaxUSDPerMin {
		return true, fmt.Sprintf("spend velocity $%.4f/min exceeds limit $%.2f/min", sum, v.MaxUSDPerMin)
	}
	if v.MaxCallsPerMin > 0 && len(v.events) > v.MaxCallsPerMin {
		return true, fmt.Sprintf("call velocity %d/min exceeds limit %d/min", len(v.events), v.MaxCallsPerMin)
	}
	return false, ""
}
