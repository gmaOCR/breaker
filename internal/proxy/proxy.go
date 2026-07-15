// Package proxy is a metering reverse proxy in front of the LLM APIs. It tees
// responses to extract token usage, feeds the breaker engine, and refuses
// further requests with HTTP 402 once the budget has tripped.
package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gmaOCR/breaker/internal/breaker"
	"github.com/gmaOCR/breaker/internal/core"
	"github.com/gmaOCR/breaker/internal/metering"
	"github.com/gmaOCR/breaker/internal/pricing"
)

// Config configures the proxy. Empty upstreams default to the real API hosts.
type Config struct {
	Session           core.SessionID
	AnthropicUpstream string
	OpenAIUpstream    string
}

// Proxy is an http.Handler.
type Proxy struct {
	engine  *breaker.Engine
	prices  *pricing.Table
	session core.SessionID
	anth    *url.URL
	oai     *url.URL
	rp      *httputil.ReverseProxy
}

// New builds a Proxy bound to a breaker engine and pricing table.
func New(engine *breaker.Engine, prices *pricing.Table, cfg Config) (*Proxy, error) {
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
	p := &Proxy{engine: engine, prices: prices, session: cfg.Session, anth: anth, oai: oai}
	p.rp = &httputil.ReverseProxy{
		Director:       p.director,
		ModifyResponse: p.modifyResponse,
		ErrorHandler:   p.errorHandler,
	}
	return p, nil
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	prov := providerForPath(r.URL.Path)
	if p.engine.Tripped() {
		writeBudgetError(w, prov, p.engine.Reason())
		return
	}
	if prov == core.ProviderOpenAI {
		p.maybeInjectUsage(r)
	}
	p.rp.ServeHTTP(w, r)
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
	record := func(u core.Usage, model string, ok bool) {
		if !ok {
			return
		}
		cost, matched := p.prices.Cost(model, u)
		p.engine.Record(core.SpendEvent{
			Session:   p.session,
			Provider:  prov,
			Model:     model,
			Usage:     u,
			CostUSD:   cost,
			Estimated: !matched,
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
	record(u, model, ok)
	resp.Body = io.NopCloser(bytes.NewReader(body))
	resp.ContentLength = int64(len(body))
	resp.Header.Set("Content-Length", strconv.Itoa(len(body)))
	return nil
}

func (p *Proxy) errorHandler(w http.ResponseWriter, r *http.Request, err error) {
	if p.engine.Tripped() {
		writeBudgetError(w, providerForPath(r.URL.Path), p.engine.Reason())
		return
	}
	w.WriteHeader(http.StatusBadGateway)
	_, _ = io.WriteString(w, "breaker: upstream error: "+err.Error())
}

func (p *Proxy) maybeInjectUsage(r *http.Request) {
	if r.Body == nil {
		return
	}
	body, err := io.ReadAll(r.Body)
	_ = r.Body.Close()
	if err != nil {
		r.Body = io.NopCloser(bytes.NewReader(nil))
		return
	}
	nb := metering.InjectUsageOptions(body)
	r.Body = io.NopCloser(bytes.NewReader(nb))
	r.ContentLength = int64(len(nb))
	r.Header.Set("Content-Length", strconv.Itoa(len(nb)))
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
