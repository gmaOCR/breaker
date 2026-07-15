package policy

import (
	"testing"
	"time"

	"github.com/gmaOCR/breaker/internal/core"
)

func hev(sec int, hash string) core.SpendEvent {
	return core.SpendEvent{At: base.Add(time.Duration(sec) * time.Second), ReqHash: hash}
}

func TestDedupTripsOnRepeat(t *testing.T) {
	d := NewDedup(3) // more than 3 identical/min trips
	for i, sec := range []int{0, 1, 2} {
		if trip, _ := d.Check(State{}, hev(sec, "h")); trip {
			t.Fatalf("tripped at repeat %d (still within 3)", i+1)
		}
	}
	if trip, _ := d.Check(State{}, hev(3, "h")); !trip {
		t.Fatal("did not trip on the 4th identical request")
	}
	// A different request is tracked independently.
	if trip, _ := d.Check(State{}, hev(4, "other")); trip {
		t.Fatal("a distinct request tripped")
	}
}

func TestDedupIgnoresEmptyHash(t *testing.T) {
	d := NewDedup(1)
	if trip, _ := d.Check(State{}, core.SpendEvent{ReqHash: ""}); trip {
		t.Fatal("empty hash must never trip")
	}
}
