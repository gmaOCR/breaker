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
| `--prices <file>` | — | Pricing override JSON (shallow-merged over the embedded table). |

At least one of `--budget` / `--tokens` is required. Exit code is the child's own
exit code, or `137` when the breaker killed the run.

```console
breaker run --budget 2.50 -- claude -p "add tests"
breaker run --tokens 500000 --grace 1s -- python agent.py
```

## `breaker serve [flags]`

Standalone proxy + rolling-window budget + dashboard. **Not yet implemented** —
see [16 — roadmap](16-roadmap-nongoals.md).

## `breaker version`

Prints the binary version and the embedded pricing table's date.

## Pricing override file

Same shape as `internal/pricing/prices.json`; only the models you list are
overridden.

```json
{ "version": "2026-07-15-local",
  "models": { "my-self-hosted-model": {"input": 0.5, "output": 1.5} } }
```
