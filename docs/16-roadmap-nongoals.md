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
  end-to-end test.
- Anthropic + OpenAI-compatible metering (streaming and non-streaming).
- Embedded, dated pricing table with a conservative fallback.

## Roadmap

- **Use case 2 — velocity / loop guard.** Trip *early* on anomalous spend
  velocity ($/min, calls/min) or repeated near-identical requests, before the
  absolute cap is reached. Implemented as an additional `policy`.
- **Use case 3 — `breaker serve`.** A long-lived standalone proxy enforcing a
  rolling per-window budget (e.g. `--daily 50`) across all runs on a shared key —
  for CI, cron jobs, and agent fleets. Adds a small append-only JSONL store so the
  window survives restarts.
- **Dashboard.** A one-page local web UI: live spend gauge vs. budget, active
  sessions, a manual KILL button, and a trip log. Served by the binary via
  `embed.FS`.
- **Notifications.** Webhook + desktop notification on trip; a one-line post-run
  spend summary.
- **`--strict` zero-overshoot mode.** A pre-flight worst-case check
  (`spent + max_tokens × price > budget → refuse before forwarding`) for callers
  who cannot tolerate the one-response overshoot inherent to post-response
  metering.

## Known limitations

- Post-response metering: the request that crosses the cap can overshoot by up to
  one response (addressed by `--strict`, above).
- Windows kills only the direct child (no process-group kill); POSIX kills the
  whole group.
- Pricing drifts; the table is dated and overridable (`--prices`), and unknown
  models price high and are flagged `estimated` rather than silently under-billed.
