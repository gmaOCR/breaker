package metering

import (
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/gmaOCR/breaker/internal/core"
)

const openAISSE = `data: {"model":"gpt-4o","choices":[{"delta":{"content":"hi"}}]}

data: {"model":"gpt-4o","choices":[],"usage":{"prompt_tokens":1200,"completion_tokens":800,"prompt_tokens_details":{"cached_tokens":300}}}

data: [DONE]

`

func TestOpenAIMeterStreaming(t *testing.T) {
	var got core.Usage
	var model string
	var ok bool
	r := NewOpenAIMeter(io.NopCloser(strings.NewReader(openAISSE)),
		func(u core.Usage, m string, complete bool, _ int64) { got, model, ok = u, m, complete })
	out, _ := io.ReadAll(r)
	if string(out) != openAISSE {
		t.Fatal("meter altered the streamed bytes")
	}
	if !ok || got.InputTokens != 1200 || got.OutputTokens != 800 || got.CacheReadTokens != 300 || model != "gpt-4o" {
		t.Fatalf("usage=%+v model=%q ok=%v", got, model, ok)
	}
}

func TestParseOpenAIJSON(t *testing.T) {
	u, m, ok := ParseOpenAIJSON([]byte(`{"model":"gpt-4o","usage":{"prompt_tokens":10,"completion_tokens":20}}`))
	if !ok || u.InputTokens != 10 || u.OutputTokens != 20 || m != "gpt-4o" {
		t.Fatalf("%+v %q %v", u, m, ok)
	}
	if _, _, ok := ParseOpenAIJSON([]byte(`{"model":"x"}`)); ok {
		t.Fatal("missing usage must report ok=false")
	}
}

func TestParseAnthropicJSON(t *testing.T) {
	u, m, ok := ParseAnthropicJSON([]byte(`{"model":"claude-opus-4-8","usage":{"input_tokens":5,"output_tokens":7,"cache_read_input_tokens":2}}`))
	if !ok || u.InputTokens != 5 || u.OutputTokens != 7 || u.CacheReadTokens != 2 || m != "claude-opus-4-8" {
		t.Fatalf("%+v %q %v", u, m, ok)
	}
	if _, _, ok := ParseAnthropicJSON([]byte(`{}`)); ok {
		t.Fatal("missing usage must report ok=false")
	}
}

func TestInjectUsageOptions(t *testing.T) {
	out := InjectUsageOptions([]byte(`{"model":"gpt-4o","stream":true}`))
	var m map[string]json.RawMessage
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatal(err)
	}
	so, ok := m["stream_options"]
	if !ok {
		t.Fatal("stream_options not injected")
	}
	var opts map[string]any
	_ = json.Unmarshal(so, &opts)
	if opts["include_usage"] != true {
		t.Fatalf("include_usage=%v", opts["include_usage"])
	}
	// Non-streaming requests are returned untouched.
	in := []byte(`{"model":"gpt-4o"}`)
	if string(InjectUsageOptions(in)) != string(in) {
		t.Fatal("non-streaming request was modified")
	}
}

func TestEstimateUsage(t *testing.T) {
	u := EstimateUsage(400, 800)
	if u.InputTokens != 100 || u.OutputTokens != 200 {
		t.Fatalf("%+v", u)
	}
	if EstimateUsage(0, 2).OutputTokens < 1 {
		t.Fatal("a non-empty response must estimate at least 1 output token")
	}
}
