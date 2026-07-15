// Package notify emits a one-shot alert when the breaker trips: an optional
// webhook POST and/or a best-effort desktop notification. All sends are
// best-effort — failures never affect enforcement.
package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"runtime"
	"time"

	"github.com/gmaOCR/breaker/internal/core"
)

// Notifier fans a trip out to the configured channels. A nil *Notifier is a
// no-op, so callers can hold one unconditionally.
type Notifier struct {
	webhookURL string
	desktop    bool
	client     *http.Client
}

// New returns a Notifier, or nil when nothing is configured.
func New(webhookURL string, desktop bool) *Notifier {
	if webhookURL == "" && !desktop {
		return nil
	}
	return &Notifier{webhookURL: webhookURL, desktop: desktop, client: &http.Client{Timeout: 5 * time.Second}}
}

// OnTrip delivers the trip to every configured channel.
func (n *Notifier) OnTrip(r core.TripReason) {
	if n == nil {
		return
	}
	if n.webhookURL != "" {
		n.postWebhook(r)
	}
	if n.desktop {
		notifyDesktop("breaker: budget tripped", r.Message)
	}
}

func (n *Notifier) postWebhook(r core.TripReason) {
	payload, _ := json.Marshal(map[string]any{
		"event":      "breaker.tripped",
		"policy":     r.Policy,
		"message":    r.Message,
		"spent_usd":  r.SpentUSD,
		"budget_usd": r.BudgetUSD,
	})
	req, err := http.NewRequest(http.MethodPost, n.webhookURL, bytes.NewReader(payload))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if resp, err := n.client.Do(req); err == nil {
		_ = resp.Body.Close()
	}
}

func notifyDesktop(title, body string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("osascript", "-e", fmt.Sprintf("display notification %q with title %q", body, title))
	case "linux":
		cmd = exec.Command("notify-send", title, body)
	default:
		return
	}
	_ = cmd.Run() // best-effort: notify-send / osascript may be absent
}
