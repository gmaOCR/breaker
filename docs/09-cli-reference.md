# 09 — CLI reference

## `breaker run [flags] -- <command> [args...]`

Wrap a command with a hard per-run budget. The proxy address is injected into the
child's environment (`ANTHROPIC_BASE_URL`, `OPENAI_BASE_URL`, `OPENAI_API_BASE`,
`BREAKER_SESSION`); the agent needs no code changes.

| Flag | Default | Meaning |
|---|---|---|
| `--budget <usd>` | `0` (off) | Hard USD cap for the run. |
| `--tokens <n>` | `0` (off) | Hard token cap for the run. |
| `--grace <dur>` | `3s` | Delay between SIGTERM and SIGKILL. |
| `--port <n>` | `0` (random) | Proxy port. |
| `--anthropic-upstream <url>` | `https://api.anthropic.com` | Override the Anthropic upstream. |
| `--openai-upstream <url>` | `https://api.openai.com` | Override the OpenAI upstream. |
| `--max-per-min <usd>` | `0` (off) | Velocity guard: trip if spend exceeds this USD/minute (fires before the absolute cap). |
| `--max-calls-per-min <n>` | `0` (off) | Velocity guard: trip if calls exceed this per minute. |
| `--prices <file>` | — | Pricing override JSON (shallow-merged over the embedded table). |

At least one of `--budget` / `--tokens` is required. Exit code is the child's own
exit code, or `137` when the breaker killed the run.

```console
breaker run --budget 2.50 -- claude -p "add tests"
breaker run --tokens 500000 --grace 1s -- python agent.py
```

## `breaker serve [flags]`

Long-lived proxy + rolling-window budget + dashboard, shared across every run that
points at it. The proxy (`/v1/*`) and the dashboard (`/`, `/api/state`, `/kill`)
share one port.

| Flag | Default | Meaning |
|---|---|---|
| `--daily <usd>` | `0` | Rolling 24h budget. |
| `--hourly <usd>` | `0` | Rolling 1h budget (takes precedence over `--daily`). |
| `--port <n>` | `8900` | Listen port (proxy + dashboard). |
| `--journal <file>` | — | JSONL spend journal; persists the rolling window across restarts. |
| `--prices <file>` | — | Pricing override JSON. |
| `--anthropic-upstream` / `--openai-upstream` | real hosts | Override upstreams. |

Set exactly one of `--daily` / `--hourly`. Once the window's spend ≥ budget, every
request is refused with 402 until older spend ages out. Sessions are grouped by the
`X-Breaker-Session` header, else a hash of the API key, else the remote address.

```console
breaker serve --daily 50 --journal ~/.breaker/spend.jsonl
export ANTHROPIC_BASE_URL=http://127.0.0.1:8900   # in each agent's env
```

## `breaker version`

Prints the binary version and the embedded pricing table's date.

## Pricing override file

Same shape as `internal/pricing/prices.json`; only the models you list are
overridden.

```json
{ "version": "2026-07-15-local",
  "models": { "my-self-hosted-model": {"input": 0.5, "output": 1.5} } }
```
