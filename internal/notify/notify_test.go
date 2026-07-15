package notify

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gmaOCR/breaker/internal/core"
)

func TestNilWhenUnconfigured(t *testing.T) {
	n := New("", false)
	if n != nil {
		t.Fatal("New with no channels should return nil")
	}
	n.OnTrip(core.TripReason{}) // must be nil-safe
}

func TestWebhookPost(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
	}))
	defer srv.Close()

	n := New(srv.URL, false)
	if n == nil {
		t.Fatal("expected a notifier")
	}
	// OnTrip is synchronous (blocks on the HTTP round-trip), so got is set after.
	n.OnTrip(core.TripReason{Policy: "hardcap", Message: "over", SpentUSD: 5, BudgetUSD: 5})

	if got["event"] != "breaker.tripped" || got["policy"] != "hardcap" || got["message"] != "over" {
		t.Fatalf("webhook payload=%v", got)
	}
}
