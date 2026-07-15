package store

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/gmaOCR/breaker/internal/core"
)

func TestWindowSumAndSessions(t *testing.T) {
	s, err := Open("", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	s.Add(Event{At: now, Session: "a", CostUSD: 1})
	s.Add(Event{At: now, Session: "b", CostUSD: 2})
	s.Add(Event{At: now.Add(-90 * time.Minute), Session: "a", CostUSD: 5}) // outside the 1h window

	if got := s.WindowSum(time.Hour); got != 3 {
		t.Fatalf("WindowSum=%v want 3", got)
	}
	sums := s.SessionSums(time.Hour)
	if sums[core.SessionID("a")] != 1 || sums[core.SessionID("b")] != 2 {
		t.Fatalf("SessionSums=%v", sums)
	}
	if r := s.Recent(2); len(r) != 2 {
		t.Fatalf("Recent(2) len=%d", len(r))
	}
}

func TestJournalPersistsAcrossReopen(t *testing.T) {
	p := filepath.Join(t.TempDir(), "spend.jsonl")
	s, err := Open(p, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	s.Add(Event{At: time.Now(), Session: "x", CostUSD: 1.5})
	_ = s.Close()

	s2, err := Open(p, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if got := s2.WindowSum(time.Hour); got != 1.5 {
		t.Fatalf("replayed WindowSum=%v want 1.5", got)
	}
}
