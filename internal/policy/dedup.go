package policy

import (
	"fmt"
	"sync"
	"time"

	"github.com/gmaOCR/breaker/internal/core"
)

// Dedup trips when the same request (by body fingerprint) repeats more than
// MaxRepeats times within a rolling minute — a strong loop signal that catches
// tight retry loops even when their per-call cost is small. Zero disables it.
type Dedup struct {
	MaxRepeats int

	window time.Duration
	mu     sync.Mutex
	seen   map[string][]time.Time
}

// NewDedup builds a dedup guard tripping above maxRepeats identical requests/min.
func NewDedup(maxRepeats int) *Dedup {
	return &Dedup{MaxRepeats: maxRepeats, window: time.Minute, seen: map[string][]time.Time{}}
}

func (d *Dedup) Name() string { return "dedup" }

func (d *Dedup) Check(_ State, ev core.SpendEvent) (bool, string) {
	if d.MaxRepeats <= 0 || ev.ReqHash == "" {
		return false, ""
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	now := ev.At
	if now.IsZero() {
		now = time.Now()
	}
	ts := append(d.seen[ev.ReqHash], now)
	cut := now.Add(-d.window)
	drop := 0
	for drop < len(ts) && ts[drop].Before(cut) {
		drop++
	}
	ts = ts[drop:]
	d.seen[ev.ReqHash] = ts
	if len(ts) > d.MaxRepeats {
		return true, fmt.Sprintf("identical request repeated %d× within a minute (loop?)", len(ts))
	}
	return false, ""
}
