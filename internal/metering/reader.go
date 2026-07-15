// Package metering extracts token usage from proxied LLM responses (streaming
// SSE and non-streaming JSON), for both the Anthropic and OpenAI wire formats.
package metering

import (
	"bytes"
	"io"

	"github.com/gmaOCR/breaker/internal/core"
)

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
	onDone func(core.Usage, string, bool)
	done   bool
}

func (m *meterReader) Read(p []byte) (int, error) {
	n, err := m.src.Read(p)
	if n > 0 {
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
	m.onDone(u, model, ok)
}

func (m *meterReader) Close() error {
	m.finish()
	return m.src.Close()
}
