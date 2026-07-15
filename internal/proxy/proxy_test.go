package proxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gmaOCR/breaker/internal/core"
	"github.com/gmaOCR/breaker/internal/pricing"
)

type fakeGuard struct {
	mu     sync.Mutex
	events []core.SpendEvent
	allow  bool
	reason core.TripReason
}

func (f *fakeGuard) Allowed() (bool, core.TripReason) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.allow, f.reason
}

func (f *fakeGuard) Record(ev core.SpendEvent) (bool, core.TripReason) {
	f.mu.Lock()
	f.events = append(f.events, ev)
	f.mu.Unlock()
	return false, core.TripReason{}
}

func (f *fakeGuard) got() []core.SpendEvent {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]core.SpendEvent(nil), f.events...)
}

func newProxy(t *testing.T, guard Guard, upstream string) *httptest.Server {
	t.Helper()
	prices, err := pricing.Load("")
	if err != nil {
		t.Fatal(err)
	}
	p, err := New(guard, prices, Config{AnthropicUpstream: upstream, OpenAIUpstream: upstream})
	if err != nil {
		t.Fatal(err)
	}
	return httptest.NewServer(p)
}

func sseServer(body string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, body)
	}))
}

const anthUsageSSE = "event: message_start\n" +
	`data: {"type":"message_start","message":{"model":"claude-opus-4-8","usage":{"input_tokens":1000,"output_tokens":1}}}` + "\n\n" +
	"event: message_delta\n" +
	`data: {"type":"message_delta","usage":{"output_tokens":500}}` + "\n\n" +
	"event: message_stop\n" + `data: {"type":"message_stop"}` + "\n\n"

const anthNoUsageSSE = "event: message_start\n" +
	`data: {"type":"message_start","message":{"model":"claude-opus-4-8"}}` + "\n\n" +
	"event: message_stop\n" + `data: {"type":"message_stop"}` + "\n\n"

const openAIUsageSSE = `data: {"model":"gpt-4o","choices":[{"delta":{"content":"hi"}}]}` + "\n\n" +
	`data: {"model":"gpt-4o","choices":[],"usage":{"prompt_tokens":1000,"completion_tokens":800}}` + "\n\n" +
	"data: [DONE]\n\n"

func post(t *testing.T, url, key string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, url, strings.NewReader(`{"model":"x","stream":true}`))
	if key != "" {
		req.Header.Set("x-api-key", key)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestProxyMetersAnthropic(t *testing.T) {
	up := sseServer(anthUsageSSE)
	defer up.Close()
	g := &fakeGuard{allow: true}
	px := newProxy(t, g, up.URL)
	defer px.Close()

	resp := post(t, px.URL+"/v1/messages", "sk-a")
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()

	evs := g.got()
	if len(evs) != 1 {
		t.Fatalf("recorded %d events, want 1", len(evs))
	}
	e := evs[0]
	if e.Model != "claude-opus-4-8" || e.Usage.InputTokens != 1000 || e.Usage.OutputTokens != 500 {
		t.Fatalf("event=%+v", e)
	}
	if e.CostUSD <= 0 || e.Estimated {
		t.Fatalf("cost=%v estimated=%v (want >0, false)", e.CostUSD, e.Estimated)
	}
	if !strings.HasPrefix(string(e.Session), "key:") {
		t.Fatalf("session=%q (want key: hash)", e.Session)
	}
	if e.ReqHash == "" {
		t.Fatal("request not fingerprinted")
	}
}

func TestProxyMetersOpenAI(t *testing.T) {
	up := sseServer(openAIUsageSSE)
	defer up.Close()
	g := &fakeGuard{allow: true}
	px := newProxy(t, g, up.URL)
	defer px.Close()

	resp := post(t, px.URL+"/v1/chat/completions", "sk-o")
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()

	evs := g.got()
	if len(evs) != 1 || evs[0].Model != "gpt-4o" ||
		evs[0].Usage.InputTokens != 1000 || evs[0].Usage.OutputTokens != 800 {
		t.Fatalf("events=%+v", evs)
	}
}

func TestProxyRefuses402(t *testing.T) {
	up := sseServer(anthUsageSSE)
	defer up.Close()
	g := &fakeGuard{allow: false, reason: core.TripReason{Message: "over budget"}}
	px := newProxy(t, g, up.URL)
	defer px.Close()

	resp := post(t, px.URL+"/v1/messages", "sk-a")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusPaymentRequired {
		t.Fatalf("status=%d want 402", resp.StatusCode)
	}
	if len(g.got()) != 0 {
		t.Fatal("a refused request must not reach the upstream / be metered")
	}
}

func TestProxyEstimatesWhenNoUsage(t *testing.T) {
	up := sseServer(anthNoUsageSSE)
	defer up.Close()
	g := &fakeGuard{allow: true}
	px := newProxy(t, g, up.URL)
	defer px.Close()

	resp := post(t, px.URL+"/v1/messages", "sk-a")
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()

	evs := g.got()
	if len(evs) != 1 {
		t.Fatalf("recorded %d events", len(evs))
	}
	if !evs[0].Estimated || evs[0].CostUSD <= 0 {
		t.Fatalf("no-usage response must be estimated with non-zero cost: %+v", evs[0])
	}
}
