package main

import (
	"testing"
	"time"

	"github.com/gmaOCR/breaker/internal/core"
	"github.com/gmaOCR/breaker/internal/store"
)

func TestWindowGuard(t *testing.T) {
	st, err := store.Open("", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	g := &windowGuard{store: st, budgetUSD: 0.05, window: time.Hour, label: "daily"}

	if ok, _ := g.Allowed(); !ok {
		t.Fatal("should allow with no spend")
	}
	g.Record(core.SpendEvent{At: time.Now(), CostUSD: 0.03, Session: "s"})
	if ok, _ := g.Allowed(); !ok {
		t.Fatal("should allow under budget ($0.03 < $0.05)")
	}
	g.Record(core.SpendEvent{At: time.Now(), CostUSD: 0.03, Session: "s"}) // $0.06 ≥ $0.05
	ok, reason := g.Allowed()
	if ok || reason.Policy != "window" {
		t.Fatalf("should refuse over budget; ok=%v reason=%+v", ok, reason)
	}

	stt := g.State()
	if stt.SpentUSD < 0.05 || stt.BudgetUSD != 0.05 || len(stt.Sessions) != 1 {
		t.Fatalf("state=%+v", stt)
	}

	g.Kill("boom")
	ok, reason = g.Allowed()
	if ok || reason.Policy != "manual" {
		t.Fatalf("after Kill: ok=%v reason=%+v", ok, reason)
	}
	if !g.State().Killed {
		t.Fatal("state should report killed")
	}
}
