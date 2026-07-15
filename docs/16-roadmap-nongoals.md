# 16 — Roadmap & non-goals

## The honesty boundary (non-goal)

**`breaker` does not enforce cloud/serverless provider spend.** AWS, Cloudflare,
and Vercel do not expose a real-time primitive that hard-stops a running function
at a cost threshold — their "budgets" are periodic alerts. `breaker` can only
enforce spend where it controls the request loop: the **LLM API calls of an agent
process it wraps**. Any claim to hard-cap a cloud bill in real time is, today,
not truthful — and this project will not make it. If cloud-side enforcement ever
becomes possible (a provider ships a real kill primitive), it would be a separate,
clearly-scoped feature.

## Built (v0.x)

- **Use case 1 — per-run hard cap.** `breaker run --budget/--tokens -- <cmd>`
  meters the wrapped agent's LLM spend and SIGKILLs it at the cap. Proven by an
  end-to-end kill test.
- **Use case 2 — velocity guard.** `--max-per-min` / `--max-calls-per-min` trip
  *early*, before the absolute cap, on a spike in spend or call rate over a
  rolling minute — catching loops before they run up the full budget.
- **Use case 3 — `breaker serve`.** A long-lived proxy enforcing a rolling
  per-window budget (`--daily` / `--hourly`) shared across all runs on a key, for
  CI / cron / fleets. Append-only JSONL journal so the window survives restarts.
  Proven by an end-to-end test (402 once the window budget is crossed).
- **Dashboard.** One-page embedded web UI served by `serve`: live spend gauge vs.
  budget, per-session breakdown, activity log, and a manual KILL button.
- Anthropic + OpenAI-compatible metering (streaming and non-streaming); embedded,
  dated pricing table with a conservative fallback.

## Roadmap

- **Notifications.** Webhook + desktop notification on trip; a one-line post-run
  spend summary.
- **`--strict` zero-overshoot mode.** A pre-flight worst-case check
  (`spent + max_tokens × price > budget → refuse before forwarding`) for callers
  who cannot tolerate the one-response overshoot inherent to post-response
  metering.
- **Request-dedup loop detection.** Hash request bodies to catch identical-request
  loops specifically (velocity already catches high-rate loops).

## Known limitations

- Post-response metering: the request that crosses the cap can overshoot by up to
  one response (addressed by `--strict`, above).
- Windows kills only the direct child (no process-group kill); POSIX kills the
  whole group.
- Pricing drifts; the table is dated and overridable (`--prices`), and unknown
  models price high and are flagged `estimated` rather than silently under-billed.
