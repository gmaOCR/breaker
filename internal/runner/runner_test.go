//go:build unix

package runner

import (
	"context"
	"testing"
	"time"

	"github.com/gmaOCR/breaker/internal/core"
)

func TestRunNormalExit(t *testing.T) {
	code, _, err := Run(context.Background(), []string{"true"}, nil, make(chan core.TripReason), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Fatalf("exit code=%d want 0", code)
	}
}

func TestRunNonzeroExit(t *testing.T) {
	code, _, err := Run(context.Background(), []string{"false"}, nil, make(chan core.TripReason), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if code != 1 {
		t.Fatalf("exit code=%d want 1", code)
	}
}

func TestRunTripKillsChild(t *testing.T) {
	trips := make(chan core.TripReason, 1)
	go func() {
		time.Sleep(100 * time.Millisecond)
		trips <- core.TripReason{Message: "boom"}
	}()
	start := time.Now()
	code, reason, err := Run(context.Background(), []string{"sleep", "30"}, nil, trips, 200*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if code != TripExitCode {
		t.Fatalf("exit code=%d want %d", code, TripExitCode)
	}
	if reason.Message != "boom" {
		t.Fatalf("reason=%q", reason.Message)
	}
	if time.Since(start) > 5*time.Second {
		t.Fatal("kill took far too long — sleep 30 was not terminated")
	}
}
