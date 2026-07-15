package metering

import (
	"bytes"
	"io"
	"testing"

	"github.com/gmaOCR/breaker/internal/core"
)

// FuzzMetering asserts the parsers never panic on arbitrary bytes. Metering is
// the make-or-break path and it is fed untrusted upstream response bytes, so a
// malformed stream must degrade to "no usage" (→ estimate), never crash.
func FuzzMetering(f *testing.F) {
	f.Add([]byte("event: message_start\n" +
		`data: {"type":"message_start","message":{"model":"x","usage":{"input_tokens":5,"output_tokens":1}}}` + "\n\n" +
		"event: message_delta\n" + `data: {"type":"message_delta","usage":{"output_tokens":9}}` + "\n\n"))
	f.Add([]byte(`data: {"model":"gpt-4o","choices":[],"usage":{"prompt_tokens":3,"completion_tokens":4}}` + "\n\ndata: [DONE]\n\n"))
	f.Add([]byte("data: not-json\n\n"))
	f.Add([]byte(""))
	f.Add([]byte("{}"))

	sink := func(core.Usage, string, bool, int64) {}
	f.Fuzz(func(_ *testing.T, data []byte) {
		_, _ = io.ReadAll(NewAnthropicMeter(io.NopCloser(bytes.NewReader(data)), sink))
		_, _ = io.ReadAll(NewOpenAIMeter(io.NopCloser(bytes.NewReader(data)), sink))
		ParseAnthropicJSON(data)
		ParseOpenAIJSON(data)
		InjectUsageOptions(data)
		_ = EstimateUsage(int64(len(data)), int64(len(data)))
	})
}
