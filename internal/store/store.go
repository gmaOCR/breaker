// Package store is the rolling-window spend store for `serve`: an in-memory
// event slice plus an optional append-only JSONL journal, so a rolling budget
// survives process restarts. Safe for concurrent use.
package store

import (
	"bufio"
	"encoding/json"
	"os"
	"sync"
	"time"

	"github.com/gmaOCR/breaker/internal/core"
)

// Event is one metered LLM response.
type Event struct {
	At        time.Time      `json:"at"`
	Session   core.SessionID `json:"session"`
	Model     string         `json:"model"`
	CostUSD   float64        `json:"cost_usd"`
	Estimated bool           `json:"estimated,omitempty"`
}

// Store retains events within `keep` and appends them to an optional journal.
type Store struct {
	mu     sync.Mutex
	events []Event
	keep   time.Duration
	f      *os.File
}

// Open loads any existing journal (dropping events older than keep) and opens it
// for appends. An empty path yields a memory-only store.
func Open(path string, keep time.Duration) (*Store, error) {
	s := &Store{keep: keep}
	if path == "" {
		return s, nil
	}
	if r, err := os.Open(path); err == nil {
		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
		cut := time.Now().Add(-keep)
		for sc.Scan() {
			if len(sc.Bytes()) == 0 {
				continue
			}
			var ev Event
			if json.Unmarshal(sc.Bytes(), &ev) == nil && ev.At.After(cut) {
				s.events = append(s.events, ev)
			}
		}
		_ = r.Close()
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	s.f = f
	return s, nil
}

// Add records an event, prunes anything older than keep, and appends to the journal.
func (s *Store) Add(ev Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, ev)
	cut := ev.At.Add(-s.keep)
	drop := 0
	for drop < len(s.events) && s.events[drop].At.Before(cut) {
		drop++
	}
	s.events = s.events[drop:]
	if s.f != nil {
		if b, err := json.Marshal(ev); err == nil {
			_, _ = s.f.Write(append(b, '\n'))
		}
	}
	// ponytail: journal is append-only and only compacted on startup — it grows
	// unbounded within a single long-lived process. Add size-triggered rewrite
	// if a serve process is meant to run for months.
}

// WindowSum returns total USD spent within the last d.
func (s *Store) WindowSum(d time.Duration) float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	cut := time.Now().Add(-d)
	var sum float64
	for _, e := range s.events {
		if e.At.After(cut) {
			sum += e.CostUSD
		}
	}
	return sum
}

// SessionSums returns per-session USD spent within the last d.
func (s *Store) SessionSums(d time.Duration) map[core.SessionID]float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	cut := time.Now().Add(-d)
	m := map[core.SessionID]float64{}
	for _, e := range s.events {
		if e.At.After(cut) {
			m[e.Session] += e.CostUSD
		}
	}
	return m
}

// Recent returns up to n most-recent events, newest first.
func (s *Store) Recent(n int) []Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Event, 0, n)
	for i := len(s.events) - 1; i >= 0 && len(out) < n; i-- {
		out = append(out, s.events[i])
	}
	return out
}

// Close closes the journal, if any.
func (s *Store) Close() error {
	if s.f != nil {
		return s.f.Close()
	}
	return nil
}
