// Package dashboard serves the one-page local UI for `serve`: a live spend gauge
// versus the rolling budget, per-session breakdown, a recent-activity log, and a
// manual KILL button. The UI is embedded in the binary.
package dashboard

import (
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"
)

//go:embed web
var webFS embed.FS

// SessionStat is one session's spend within the current window.
type SessionStat struct {
	Session string  `json:"session"`
	USD     float64 `json:"usd"`
}

// RecentEvent is one metered response, for the activity log.
type RecentEvent struct {
	At        string  `json:"at"`
	Session   string  `json:"session"`
	Model     string  `json:"model"`
	USD       float64 `json:"usd"`
	Estimated bool    `json:"estimated"`
}

// State is the dashboard's snapshot of the guard.
type State struct {
	WindowLabel string        `json:"window_label"`
	BudgetUSD   float64       `json:"budget_usd"`
	SpentUSD    float64       `json:"spent_usd"`
	Killed      bool          `json:"killed"`
	Sessions    []SessionStat `json:"sessions"`
	Recent      []RecentEvent `json:"recent"`
}

// Controller is what the dashboard reads and controls; serve's guard implements it.
type Controller interface {
	State() State
	Kill(reason string)
}

// Handler returns the dashboard HTTP handler: UI at /, JSON at /api/state,
// POST /kill to trip the guard manually.
func Handler(c Controller) http.Handler {
	sub, _ := fs.Sub(webFS, "web")
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(sub)))
	mux.HandleFunc("/api/state", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(c.State())
	})
	mux.HandleFunc("/kill", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		c.Kill("manual kill from dashboard")
		w.WriteHeader(http.StatusNoContent)
	})
	return mux
}
