# 11 — Config reference

Full flag tables are in [09 — CLI reference](09-cli-reference.md). This page covers
configuration that isn't a flag.

## Environment variables the proxy injects (run mode)

When `breaker run` launches the child, it sets these so the agent talks to the
proxy instead of the real API — no agent code change:

| Variable | Value |
|---|---|
| `ANTHROPIC_BASE_URL` | `http://127.0.0.1:<proxy-port>` |
| `OPENAI_BASE_URL` | `http://127.0.0.1:<proxy-port>/v1` |
| `OPENAI_API_BASE` | same as `OPENAI_BASE_URL` (older SDKs) |
| `BREAKER_SESSION` | the run's session id |

In `serve` mode you set `ANTHROPIC_BASE_URL` / `OPENAI_BASE_URL` yourself, pointing
at the serve address.

## Upstreams

Default upstreams are `https://api.anthropic.com` and `https://api.openai.com`.
Override with `--anthropic-upstream` / `--openai-upstream` (any OpenAI-compatible
host, or a mock for testing).

## Pricing override

`--prices <file>` — see [03 — pricing](03-pricing.md).

## API keys

`breaker` does not manage keys. The agent's existing `ANTHROPIC_API_KEY` /
`OPENAI_API_KEY` are forwarded untouched to the upstream — see
[14 — security & keys](14-security-keys.md).
