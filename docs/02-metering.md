# 02 — Metering

`breaker` derives spend from the tokens each response actually reports. The proxy
tees the response body through a provider-specific parser (`internal/metering`)
without buffering the whole stream.

## Anthropic (Messages API)

Streaming SSE carries usage across events:

- `message_start` → `message.usage.{input_tokens, cache_creation_input_tokens, cache_read_input_tokens}` and the model name.
- `message_delta` → cumulative `usage.output_tokens` (the last one before `message_stop` is final).

Non-streaming responses carry the same `usage` object in the JSON body
(`ParseAnthropicJSON`).

## OpenAI-compatible (Chat Completions)

Streaming responses omit usage **unless** the request sets
`stream_options.include_usage=true`. The proxy injects that field into outgoing
streaming requests (`InjectUsageOptions`); the usage then arrives in a final chunk
with empty `choices`. Non-streaming responses carry `usage.{prompt_tokens,
completion_tokens}` directly (`ParseOpenAIJSON`).

## Fallback estimate (never silent-zero)

If a completed response reports **no** usage (e.g. an OpenAI-compatible server that
ignores `include_usage`), the proxy estimates from raw byte sizes —
`EstimateUsage(reqBytes, respBytes)` at ~4 bytes/token — and flags the event
`Estimated`. This is deliberately non-zero: a response that produced output is
always metered, so the breaker still trips. Estimation is labelled, never hidden.
