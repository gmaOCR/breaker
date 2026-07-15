# breaker

**A cost circuit-breaker for AI agents.** Set a hard dollar budget on an agent run; `breaker` kills the run *before* it burns through your budget — the protection your LLM provider doesn't give you.

> Provider "spend limits" alert you minutes-to-hours *after* the money is gone. `breaker` enforces a hard cap in real time, on the one place it actually can: the agent's own request loop.

<!-- DEMO GIF SLOT — record: breaker run --budget 0.05 -- <looping agent>  →  TRIPPED + killed -->

```console
$ breaker run --budget 5.00 -- claude -p "refactor this whole repo"
breaker: guarding run (cap $5.00) — proxy at http://127.0.0.1:38081
...agent works...
breaker: spent $5.01 over 812k tokens
breaker: TRIPPED — budget of $5.00 reached ($5.0100 spent) [hardcap]
# the agent process is killed. you do not wake up to an $80,000 bill.
```

## Why

AI coding agents burn 10–100× the tokens of a chatbot, and a looping agent (or a runaway `while` in your own harness) can rack up a four- or five-figure bill in hours with no human watching. Every provider's "budget" is an *alert*, checked periodically and fired after the fact — not a hard stop. `breaker` is the hard stop.

It works by sitting between your agent and the LLM API as a local proxy: it meters real token usage from the responses, prices it, and when you cross the budget it (a) refuses further API calls and (b) kills the wrapped process. No provider cooperation required.

## Install

```console
go install github.com/gmaOCR/breaker/cmd/breaker@latest
```

Or build from source: `git clone … && cd breaker && make build` → `./bin/breaker`.

## Quickstart

Wrap any command that talks to Anthropic or an OpenAI-compatible API. `breaker` injects the proxy's address into the child's environment (`ANTHROPIC_BASE_URL` / `OPENAI_BASE_URL`) — the agent needs no changes.

```console
# Cap a Claude Code run at $2:
breaker run --budget 2.00 -- claude -p "add tests for the parser"

# Cap by tokens instead of dollars:
breaker run --tokens 500000 -- python my_agent.py

# Point at an OpenAI-compatible endpoint:
breaker run --budget 1.00 --openai-upstream https://api.openai.com -- python my_agent.py

# Trip EARLY on a runaway loop, before the absolute cap is even reached:
breaker run --budget 5.00 --max-calls-per-min 120 -- python my_agent.py
```

### Shared budget across many runs — `breaker serve`

Run one long-lived proxy with a rolling budget (per hour or per day) shared across every agent, CI job, and cron task that points at it, with a live web dashboard:

```console
breaker serve --daily 50 --journal ~/.breaker/spend.jsonl
# dashboard: http://127.0.0.1:8900/   (live gauge, per-session spend, KILL button)
export ANTHROPIC_BASE_URL=http://127.0.0.1:8900   # then, in each agent's env
```

Once the rolling window's spend crosses the budget, every request gets a `402` until older spend ages out of the window. `--journal` makes the window survive restarts.

When the cap trips, subsequent API calls get a clean `402` (in the provider's own error shape, so the SDK surfaces it as fatal) and the process receives `SIGTERM` → `SIGKILL`. The exit code is `137`.

## How it works

```
  your agent ──(ANTHROPIC_BASE_URL)──▶ breaker proxy ──HTTPS──▶ api.anthropic.com
                                            │  meters usage from the response
                                            │  prices it (embedded table)
                                            ▼
                                       budget engine ──trip──▶ SIGKILL the agent
```

- **Metering** parses token usage straight from the streamed responses (Anthropic `message_start`/`message_delta`; OpenAI final usage chunk — `breaker` auto-requests it).
- **Pricing** uses a dated, embedded table (`internal/pricing/prices.json`); unknown models fall back to a deliberately *high* price and are flagged `estimated`, so the breaker never fails to trip.
- **Enforcement** is post-response (usage lands at the end of a response), so the request that crosses the line finishes; every later one is refused. `--strict` adds a pre-flight worst-case check for zero overshoot *(roadmap)*.

## Honesty boundary

`breaker` enforces spend **only where it owns the request loop: AI-agent LLM calls.** It does **not** — and cannot honestly — enforce cloud/serverless provider spend (AWS/Cloudflare/Vercel don't expose a real-time kill primitive). If you see a tool claiming to hard-cap your cloud bill, be skeptical. See [`docs/16-roadmap-nongoals.md`](docs/16-roadmap-nongoals.md).

## Status

Early but functional — all three core use cases are built and covered by end-to-end tests:

1. **Per-run hard cap** — `breaker run --budget/--tokens` (proven: a runaway run is SIGKILLed at budget, exit 137).
2. **Velocity guard** — `--max-per-min` / `--max-calls-per-min` trip *before* the absolute cap, catching loops early.
3. **Rolling shared budget** — `breaker serve --daily/--hourly` + live dashboard + persistent journal (proven: 402 once the window budget is crossed).

Also built: a loop guard (`--max-repeats`), trip notifications (`--notify-webhook` / `--notify-desktop`), and a size-based usage estimator (flagged `estimated`) for providers that report no usage. On the roadmap: a `--strict` zero-overshoot mode. See [`docs/16-roadmap-nongoals.md`](docs/16-roadmap-nongoals.md).

## Providers

Anthropic Messages API and OpenAI-compatible Chat Completions (covers Claude Code, Cursor, and most agent frameworks via a `base_url` override).

## Limitations

- Post-response metering means the crossing request can overshoot by up to one response (`--strict` pre-flight mode is on the roadmap).
- Process-group kill is full on POSIX; on Windows only the direct child is killed (best-effort).
- Some OpenAI-compatible servers don't report usage even when asked; `breaker` then estimates from response size and flags the run `estimated`.

## License

Apache-2.0.
