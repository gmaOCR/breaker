package proxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gmaOCR/breaker/internal/pricing"
)

func benchPost(b *testing.B, url string) {
	b.Helper()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest(http.MethodPost, url, strings.NewReader(`{"model":"x","stream":true}`))
		req.Header.Set("x-api-key", "sk-bench")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			b.Fatal(err)
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}
}

// BenchmarkDirectUpstream is the baseline: client → upstream, no breaker.
func BenchmarkDirectUpstream(b *testing.B) {
	up := sseServer(anthUsageSSE)
	defer up.Close()
	benchPost(b, up.URL+"/v1/messages")
}

// BenchmarkThroughProxy is client → breaker proxy → upstream. The delta versus
// the baseline is breaker's added latency (metering + pricing + guard).
func BenchmarkThroughProxy(b *testing.B) {
	up := sseServer(anthUsageSSE)
	defer up.Close()
	prices, err := pricing.Load("")
	if err != nil {
		b.Fatal(err)
	}
	p, err := New(&fakeGuard{allow: true}, prices, Config{AnthropicUpstream: up.URL, OpenAIUpstream: up.URL})
	if err != nil {
		b.Fatal(err)
	}
	px := httptest.NewServer(p)
	defer px.Close()
	benchPost(b, px.URL+"/v1/messages")
}
