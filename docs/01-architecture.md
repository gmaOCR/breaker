# 01 — Architecture

Single Go binary, standard-library only. Packages under `internal/`:

| Package | Responsibility |
|---|---|
| `core` | Shared domain types (`Usage`, `SpendEvent`, `TripReason`, `Provider`). No logic — breaks import cycles. |
| `pricing` | Embedded, dated price table + cost math. `Cost(model, usage) → (usd, matched)`; unknown model → high fallback + `matched=false`. |
| `metering` | Extract token usage from proxied responses. Streaming (SSE) and non-streaming JSON, for Anthropic and OpenAI wire formats. |
| `breaker` | The budget engine: accumulates spend, runs policies, fires a one-shot trip on a channel. |
| `policy` | Pluggable trip checks. `HardCap` (USD/token budget) today; velocity/dedup on the roadmap. |
| `proxy` | `httputil.ReverseProxy` that tees responses through `metering`, records to the engine, and returns 402 once tripped. |
| `runner` | Launches the child in its own process group, injects the proxy env, and escalates SIGTERM→SIGKILL on trip. |
| `config` / `dashboard` / `notify` / `store` | Reserved for `serve`, the dashboard, notifications, and rolling-window persistence (roadmap). |

## Request path (`breaker run`)

```
child process ──ANTHROPIC_BASE_URL/OPENAI_BASE_URL──▶ in-process proxy
                                                          │ Director → real upstream (HTTPS)
                                                          │ ModifyResponse → tee body → metering
                                                          ▼
                                                     engine.Record(SpendEvent)
                                                          │ policies evaluate
                                                          ▼ (trip, one-shot)
                                                     Trips() channel
                                                          ▼
                                                     runner: SIGTERM → grace → SIGKILL (process group)
```

- **Session correlation** in `run` mode is trivial: one proxy instance per run, so
  all traffic is one budget. No header cooperation from the agent is required.
- **Enforcement is post-response.** Usage is only known once a response
  completes, so the request that crosses the line is allowed to finish; every
  subsequent request is refused with a 402 shaped like the provider's own error
  (so SDKs treat it as fatal rather than retrying).
- **Kill semantics.** The child is started with `Setpgid`, so the whole process
  group (the agent and anything it spawned) is signalled. `--grace` controls the
  SIGTERM→SIGKILL window. Exit code on a killed run is `137`.

## Metering details

- **Anthropic SSE**: input/cache tokens arrive in `message_start`; cumulative
  `output_tokens` in each `message_delta`. The meter reads them as bytes stream
  to the client, buffering nothing.
- **OpenAI SSE**: usage only appears in a final chunk when the request carries
  `stream_options.include_usage=true`; the proxy injects that field. If a server
  still reports no usage, the run is flagged `estimated`.
- **Pricing** never returns zero for real usage — a zero price would mean the
  breaker never trips, which would defeat the entire product.
