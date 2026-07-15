// Package metering extracts token usage from proxied LLM responses (streaming
// SSE and non-streaming JSON), for both the Anthropic and OpenAI wire formats.
package metering

import (
	"bytes"
	"io"

	"github.com/gmaOCR/breaker/internal/core"
)

// Sink receives the metered usage once a response completes. complete is false
// when the provider reported no usage; respBytes is the response body size, so
// the caller can fall back to a size-based estimate rather than record zero.
type Sink func(usage core.Usage, model string, complete bool, respBytes int64)

// lineParser consumes complete SSE lines (no trailing newline) and accumulates
// usage. final reports the accumulated usage once the stream ends.
type lineParser interface {
	feedLine(line []byte)
	final() (usage core.Usage, model string, complete bool)
}

// meterReader tees the upstream body to the client unchanged while feeding a
// line-oriented parser. onDone fires exactly once, at EOF or Close.
type meterReader struct {
	src    io.ReadCloser
	parser lineParser
	buf    []byte
	onDone Sink
	n      int64
	done   bool
}

func (m *meterReader) Read(p []byte) (int, error) {
	n, err := m.src.Read(p)
	if n > 0 {
		m.n += int64(n)
		m.consume(p[:n])
	}
	if err == io.EOF {
		m.finish()
	}
	return n, err
}

func (m *meterReader) consume(b []byte) {
	m.buf = append(m.buf, b...)
	for {
		i := bytes.IndexByte(m.buf, '\n')
		if i < 0 {
			return
		}
		line := m.buf[:i]
		if n := len(line); n > 0 && line[n-1] == '\r' {
			line = line[:n-1]
		}
		m.parser.feedLine(line)
		m.buf = m.buf[i+1:]
	}
}

func (m *meterReader) finish() {
	if m.done {
		return
	}
	m.done = true
	if len(m.buf) > 0 {
		m.parser.feedLine(m.buf)
		m.buf = nil
	}
	u, model, ok := m.parser.final()
	m.onDone(u, model, ok, m.n)
}

func (m *meterReader) Close() error {
	m.finish()
	return m.src.Close()
}

// EstimateUsage approximates token usage from raw byte sizes (~4 bytes/token)
// when a provider reports no usage. Deliberately non-zero when there was a
// response, so the breaker still meters (and trips) rather than under-count.
func EstimateUsage(reqBytes, respBytes int64) core.Usage {
	out := int(respBytes / 4)
	if out < 1 && respBytes > 0 {
		out = 1
	}
	return core.Usage{InputTokens: int(reqBytes / 4), OutputTokens: out}
}
