// Package proxy is a metering reverse proxy in front of the LLM APIs. It tees
// responses to extract token usage, feeds a Guard (the budget authority), and
// refuses further requests with HTTP 402 once the Guard disallows them.
package proxy

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gmaOCR/breaker/internal/core"
	"github.com/gmaOCR/breaker/internal/metering"
	"github.com/gmaOCR/breaker/internal/pricing"
)

// Guard is the budget authority the proxy consults. `run` uses a one-shot
// breaker.Engine; `serve` uses a rolling-window guard. Both implement this.
type Guard interface {
	// Allowed reports whether a new request may proceed, with the reason if not.
	Allowed() (bool, core.TripReason)
	// Record ingests a completed response's metered spend.
	Record(core.SpendEvent) (bool, core.TripReason)
}

// Config configures the proxy. Empty upstreams default to the real API hosts.
// Session, when set, pins all traffic to one budget (run mode); when empty the
// proxy derives a session per request (serve mode).
type Config struct {
	Session           core.SessionID
	AnthropicUpstream string
	OpenAIUpstream    string
}

type ctxKey int

const (
	reqHashKey ctxKey = iota
	reqBytesKey
)

// Proxy is an http.Handler.
type Proxy struct {
	guard   Guard
	prices  *pricing.Table
	session core.SessionID
	anth    *url.URL
	oai     *url.URL
	rp      *httputil.ReverseProxy

	mu     sync.Mutex
	warned map[string]bool
}

// New builds a Proxy bound to a Guard and pricing table.
func New(guard Guard, prices *pricing.Table, cfg Config) (*Proxy, error) {
	if cfg.AnthropicUpstream == "" {
		cfg.AnthropicUpstream = "https://api.anthropic.com"
	}
	if cfg.OpenAIUpstream == "" {
		cfg.OpenAIUpstream = "https://api.openai.com"
	}
	anth, err := url.Parse(cfg.AnthropicUpstream)
	if err != nil {
		return nil, fmt.Errorf("proxy: bad anthropic upstream: %w", err)
	}
	oai, err := url.Parse(cfg.OpenAIUpstream)
	if err != nil {
		return nil, fmt.Errorf("proxy: bad openai upstream: %w", err)
	}
	p := &Proxy{guard: guard, prices: prices, session: cfg.Session, anth: anth, oai: oai, warned: map[string]bool{}}
	p.rp = &httputil.ReverseProxy{
		Director:       p.director,
		ModifyResponse: p.modifyResponse,
		ErrorHandler:   p.errorHandler,
	}
	return p, nil
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	prov := providerForPath(r.URL.Path)
	if ok, reason := p.guard.Allowed(); !ok {
		writeBudgetError(w, prov, reason)
		return
	}

	// Buffer the request body once: fingerprint it (loop detection), measure it
	// (fallback estimate), and inject usage options for OpenAI streaming.
	var (
		reqBytes int64
		reqHash  string
	)
	if r.Body != nil {
		body, err := io.ReadAll(r.Body)
		_ = r.Body.Close()
		if err != nil {
			body = nil
		}
		reqBytes = int64(len(body))
		sum := sha256.Sum256(body)
		reqHash = hex.EncodeToString(sum[:])[:16]
		if prov == core.ProviderOpenAI {
			body = metering.InjectUsageOptions(body)
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		r.ContentLength = int64(len(body))
		r.Header.Set("Content-Length", strconv.Itoa(len(body)))
	}

	ctx := context.WithValue(r.Context(), reqHashKey, reqHash)
	ctx = context.WithValue(ctx, reqBytesKey, reqBytes)
	p.rp.ServeHTTP(w, r.WithContext(ctx))
}

func (p *Proxy) director(r *http.Request) {
	target := p.anth
	if providerForPath(r.URL.Path) == core.ProviderOpenAI {
		target = p.oai
	}
	r.URL.Scheme = target.Scheme
	r.URL.Host = target.Host
	r.Host = target.Host
	if base := strings.TrimRight(target.Path, "/"); base != "" {
		r.URL.Path = base + r.URL.Path
	}
}

func (p *Proxy) modifyResponse(resp *http.Response) error {
	prov := providerForPath(resp.Request.URL.Path)
	sess := p.sessionFor(resp.Request)
	hash, _ := resp.Request.Context().Value(reqHashKey).(string)
	reqBytes, _ := resp.Request.Context().Value(reqBytesKey).(int64)

	record := func(u core.Usage, model string, ok bool, respBytes int64) {
		if !ok {
			u = metering.EstimateUsage(reqBytes, respBytes)
		}
		cost, matched := p.prices.Cost(model, u)
		if !matched && model != "" {
			p.warnUnknown(model)
		}
		p.guard.Record(core.SpendEvent{
			Session:   sess,
			Provider:  prov,
			Model:     model,
			Usage:     u,
			CostUSD:   cost,
			Estimated: !ok || !matched,
			ReqHash:   hash,
			At:        time.Now(),
		})
	}

	if strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		if prov == core.ProviderOpenAI {
			resp.Body = metering.NewOpenAIMeter(resp.Body, record)
		} else {
			resp.Body = metering.NewAnthropicMeter(resp.Body, record)
		}
		return nil
	}
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return err
	}
	var (
		u     core.Usage
		model string
		ok    bool
	)
	if prov == core.ProviderOpenAI {
		u, model, ok = metering.ParseOpenAIJSON(body)
	} else {
		u, model, ok = metering.ParseAnthropicJSON(body)
	}
	record(u, model, ok, int64(len(body)))
	resp.Body = io.NopCloser(bytes.NewReader(body))
	resp.ContentLength = int64(len(body))
	resp.Header.Set("Content-Length", strconv.Itoa(len(body)))
	return nil
}

func (p *Proxy) errorHandler(w http.ResponseWriter, r *http.Request, err error) {
	if ok, reason := p.guard.Allowed(); !ok {
		writeBudgetError(w, providerForPath(r.URL.Path), reason)
		return
	}
	w.WriteHeader(http.StatusBadGateway)
	_, _ = io.WriteString(w, "breaker: upstream error: "+err.Error())
}

// warnUnknown logs once per unknown model that its spend is priced with the
// high fallback and is therefore an estimate.
func (p *Proxy) warnUnknown(model string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.warned[model] {
		return
	}
	p.warned[model] = true
	fmt.Fprintf(os.Stderr, "breaker: unknown model %q — pricing with the high fallback; spend is estimated\n", model)
}

// sessionFor attributes a request to a budget/session. Run mode pins one
// session; serve mode groups by an explicit header, else the API key hash
// (never the raw key), else the remote address.
func (p *Proxy) sessionFor(r *http.Request) core.SessionID {
	if p.session != "" {
		return p.session
	}
	if h := r.Header.Get("X-Breaker-Session"); h != "" {
		return core.SessionID(h)
	}
	if key := apiKey(r); key != "" {
		sum := sha256.Sum256([]byte(key))
		return core.SessionID("key:" + hex.EncodeToString(sum[:])[:12])
	}
	return core.SessionID(r.RemoteAddr)
}

func apiKey(r *http.Request) string {
	if k := r.Header.Get("x-api-key"); k != "" {
		return k
	}
	return strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
}

func providerForPath(path string) core.Provider {
	if strings.Contains(path, "/completions") ||
		strings.Contains(path, "/embeddings") ||
		strings.Contains(path, "/responses") {
		return core.ProviderOpenAI
	}
	return core.ProviderAnthropic
}

func writeBudgetError(w http.ResponseWriter, prov core.Provider, reason core.TripReason) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusPaymentRequired)
	msg := "breaker: budget exceeded — request refused (" + reason.Message + ")"
	var body any
	if prov == core.ProviderOpenAI {
		body = map[string]any{"error": map[string]any{
			"message": msg, "type": "breaker_budget_exceeded", "code": "budget_exceeded",
		}}
	} else {
		body = map[string]any{"type": "error", "error": map[string]any{
			"type": "breaker_budget_exceeded", "message": msg,
		}}
	}
	_ = json.NewEncoder(w).Encode(body)
}
