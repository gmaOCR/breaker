package metering

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/gmaOCR/breaker/internal/core"
)

type openAITokDetails struct {
	CachedTokens int `json:"cached_tokens"`
}

type openAIUsage struct {
	PromptTokens        int               `json:"prompt_tokens"`
	CompletionTokens    int               `json:"completion_tokens"`
	PromptTokensDetails *openAITokDetails `json:"prompt_tokens_details"`
}

type openAIChunk struct {
	Model string       `json:"model"`
	Usage *openAIUsage `json:"usage"`
}

// openAIParser reads the final usage chunk of a streamed chat completion. That
// chunk only exists when the request carried stream_options.include_usage=true
// (see InjectUsageOptions).
type openAIParser struct {
	u     core.Usage
	model string
	seen  bool
}

func (p *openAIParser) feedLine(line []byte) {
	const pfx = "data: "
	if !bytes.HasPrefix(line, []byte(pfx)) {
		return
	}
	data := bytes.TrimSpace(line[len(pfx):])
	if len(data) == 0 || bytes.Equal(data, []byte("[DONE]")) {
		return
	}
	var ev openAIChunk
	if json.Unmarshal(data, &ev) != nil {
		return
	}
	if ev.Model != "" {
		p.model = ev.Model
	}
	if ev.Usage != nil {
		p.u.InputTokens = ev.Usage.PromptTokens
		p.u.OutputTokens = ev.Usage.CompletionTokens
		if ev.Usage.PromptTokensDetails != nil {
			p.u.CacheReadTokens = ev.Usage.PromptTokensDetails.CachedTokens
		}
		p.seen = true
	}
}

func (p *openAIParser) final() (core.Usage, string, bool) {
	return p.u, p.model, p.seen
}

// NewOpenAIMeter tees an OpenAI-compatible SSE response body.
func NewOpenAIMeter(src io.ReadCloser, onDone Sink) io.ReadCloser {
	return &meterReader{src: src, parser: &openAIParser{}, onDone: onDone}
}

// ParseOpenAIJSON extracts usage from a non-streaming chat completion.
func ParseOpenAIJSON(body []byte) (core.Usage, string, bool) {
	var r struct {
		Model string       `json:"model"`
		Usage *openAIUsage `json:"usage"`
	}
	if json.Unmarshal(body, &r) != nil || r.Usage == nil {
		return core.Usage{}, "", false
	}
	u := core.Usage{InputTokens: r.Usage.PromptTokens, OutputTokens: r.Usage.CompletionTokens}
	if r.Usage.PromptTokensDetails != nil {
		u.CacheReadTokens = r.Usage.PromptTokensDetails.CachedTokens
	}
	return u, r.Model, true
}

// InjectUsageOptions adds stream_options.include_usage=true to a streaming OpenAI
// chat request so the response carries a final usage chunk. Non-streaming or
// unparseable bodies are returned unchanged.
func InjectUsageOptions(body []byte) []byte {
	var m map[string]json.RawMessage
	if json.Unmarshal(body, &m) != nil {
		return body
	}
	raw, ok := m["stream"]
	if !ok {
		return body
	}
	var streaming bool
	if json.Unmarshal(raw, &streaming) != nil || !streaming {
		return body
	}
	opts := map[string]any{"include_usage": true}
	if so, ok := m["stream_options"]; ok {
		var existing map[string]any
		if json.Unmarshal(so, &existing) == nil {
			existing["include_usage"] = true
			opts = existing
		}
	}
	b, err := json.Marshal(opts)
	if err != nil {
		return body
	}
	m["stream_options"] = b
	out, err := json.Marshal(m)
	if err != nil {
		return body
	}
	return out
}
