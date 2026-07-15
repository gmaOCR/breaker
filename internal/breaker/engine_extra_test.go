package breaker

import (
	"testing"

	"github.com/gmaOCR/breaker/internal/core"
)

func TestKillSnapshotAllowed(t *testing.T) {
	e := New(Config{BudgetUSD: 10})
	if ok, _ := e.Allowed(); !ok {
		t.Fatal("should be allowed initially")
	}
	e.Record(core.SpendEvent{CostUSD: 1, Usage: core.Usage{OutputTokens: 50}, Estimated: true})

	e.Kill("manual stop")
	if ok, _ := e.Allowed(); ok {
		t.Fatal("should be disallowed after Kill")
	}
	if !e.Tripped() {
		t.Fatal("Tripped() false after Kill")
	}
	if e.Reason().Policy != "manual" {
		t.Fatalf("reason policy=%q want manual", e.Reason().Policy)
	}
	snap := e.Snapshot()
	if snap.SpentUSD != 1 || !snap.Tripped || !snap.Estimated {
		t.Fatalf("snapshot=%+v", snap)
	}

	// Kill is idempotent — a second call must not overwrite the reason.
	e.Kill("second")
	if e.Reason().Message == "second" {
		t.Fatal("second Kill should be a no-op")
	}
}
