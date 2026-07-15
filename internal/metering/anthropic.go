package metering

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/gmaOCR/breaker/internal/core"
)

type anthropicUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
}

type anthropicEvent struct {
	Message *struct {
		Model string          `json:"model"`
		Usage *anthropicUsage `json:"usage"`
	} `json:"message"`
	Usage *anthropicUsage `json:"usage"`
}

// anthropicParser accumulates usage across the SSE event sequence: input/cache
// tokens arrive in message_start, cumulative output_tokens in each message_delta.
type anthropicParser struct {
	u     core.Usage
	model string
	seen  bool
}

func (p *anthropicParser) feedLine(line []byte) {
	const pfx = "data: "
	if !bytes.HasPrefix(line, []byte(pfx)) {
		return
	}
	var ev anthropicEvent
	if json.Unmarshal(line[len(pfx):], &ev) != nil {
		return // ping/keepalive or partial — ignore
	}
	if ev.Message != nil {
		if ev.Message.Model != "" {
			p.model = ev.Message.Model
		}
		if ev.Message.Usage != nil {
			p.apply(ev.Message.Usage)
		}
	}
	if ev.Usage != nil {
		p.apply(ev.Usage)
	}
}

func (p *anthropicParser) apply(u *anthropicUsage) {
	if u.InputTokens > 0 {
		p.u.InputTokens = u.InputTokens
	}
	if u.OutputTokens > 0 {
		p.u.OutputTokens = u.OutputTokens
	}
	if u.CacheCreationInputTokens > 0 {
		p.u.CacheWriteTokens = u.CacheCreationInputTokens
	}
	if u.CacheReadInputTokens > 0 {
		p.u.CacheReadTokens = u.CacheReadInputTokens
	}
	p.seen = true
}

func (p *anthropicParser) final() (core.Usage, string, bool) {
	return p.u, p.model, p.seen
}

// NewAnthropicMeter tees an Anthropic SSE response body, extracting token usage.
func NewAnthropicMeter(src io.ReadCloser, onDone Sink) io.ReadCloser {
	return &meterReader{src: src, parser: &anthropicParser{}, onDone: onDone}
}

// ParseAnthropicJSON extracts usage from a non-streaming Anthropic response.
func ParseAnthropicJSON(body []byte) (core.Usage, string, bool) {
	var r struct {
		Model string          `json:"model"`
		Usage *anthropicUsage `json:"usage"`
	}
	if json.Unmarshal(body, &r) != nil || r.Usage == nil {
		return core.Usage{}, "", false
	}
	return core.Usage{
		InputTokens:      r.Usage.InputTokens,
		OutputTokens:     r.Usage.OutputTokens,
		CacheWriteTokens: r.Usage.CacheCreationInputTokens,
		CacheReadTokens:  r.Usage.CacheReadInputTokens,
	}, r.Model, true
}
