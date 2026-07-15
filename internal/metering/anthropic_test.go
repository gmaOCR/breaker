package metering

import (
	"io"
	"strings"
	"testing"

	"github.com/gmaOCR/breaker/internal/core"
)

const anthropicSSE = `event: message_start
data: {"type":"message_start","message":{"model":"claude-opus-4-8","usage":{"input_tokens":1000,"output_tokens":1,"cache_read_input_tokens":200}}}

event: content_block_delta
data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"hi"}}

event: message_delta
data: {"type":"message_delta","usage":{"output_tokens":500}}

event: message_stop
data: {"type":"message_stop"}

`

func TestAnthropicMeterStreaming(t *testing.T) {
	var got core.Usage
	var gotModel string
	var ok bool
	r := NewAnthropicMeter(io.NopCloser(strings.NewReader(anthropicSSE)),
		func(u core.Usage, model string, complete bool, _ int64) {
			got, gotModel, ok = u, model, complete
		})
	// Draining the reader must yield the original bytes unchanged to the client.
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(out) != anthropicSSE {
		t.Fatal("meter reader altered the streamed bytes")
	}
	if !ok {
		t.Fatal("usage never reported")
	}
	if got.InputTokens != 1000 || got.OutputTokens != 500 || got.CacheReadTokens != 200 {
		t.Errorf("usage = %+v; want input=1000 output=500 cacheRead=200", got)
	}
	if gotModel != "claude-opus-4-8" {
		t.Errorf("model = %q; want claude-opus-4-8", gotModel)
	}
}
