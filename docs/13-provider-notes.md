# 13 — Provider notes

## Anthropic (Messages API)

- Path: `/v1/messages`. Auth header: `x-api-key`.
- Usage is emitted in the SSE stream (`message_start` + `message_delta`) and in
  non-streaming JSON. `breaker` reads it directly — no request modification.
- Agents set `ANTHROPIC_BASE_URL` to the proxy; the SDK appends `/v1/messages`.

## OpenAI-compatible (Chat Completions)

- Paths containing `/completions`, `/embeddings`, or `/responses` route to the
  OpenAI upstream.
- Streaming responses only include usage when the request carries
  `stream_options.include_usage=true`; `breaker` injects it. Servers that still
  omit usage fall back to the size estimator (flagged `estimated`).
- Works with any OpenAI-compatible host — set `--openai-upstream` (e.g. a local
  model server, Together, Groq). Agents set `OPENAI_BASE_URL` to the proxy `/v1`.

## Re-verifying the wire format

Provider SSE shapes drift. To re-check, capture a real streamed response with
`curl -N` and confirm the `usage` fields still match `internal/metering`
(`anthropic.go` / `openai.go`). The parsers ignore unknown events, so additive
changes are safe; renamed usage fields would need a parser update.

## localhost is plaintext

The agent → proxy hop is plain HTTP on `127.0.0.1`; the proxy → upstream hop is
HTTPS. Anthropic and OpenAI SDKs accept an `http://` base URL, which is why this
works with no agent changes.
