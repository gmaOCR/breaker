# Changelog

All notable changes to this project are documented here. Format loosely follows
[Keep a Changelog](https://keepachangelog.com/); versions follow SemVer.

## [Unreleased]

### Added
- `breaker run` — wrap a command with a hard per-run budget (`--budget` USD and/or
  `--tokens`); the wrapped process is SIGKILLed when the cap trips.
- Velocity guard (`--max-per-min`, `--max-calls-per-min`) — trips before the
  absolute cap on a spike in spend or call rate over a rolling minute.
- `breaker serve` — long-lived proxy enforcing a rolling per-window budget
  (`--daily` / `--hourly`) shared across runs, with an append-only JSONL journal
  (`--journal`) so the window survives restarts.
- One-page embedded web dashboard (live gauge, per-session breakdown, activity
  log, manual KILL button) served by `breaker serve`.
- Metering reverse proxy for the Anthropic Messages API and OpenAI-compatible
  Chat Completions, including streaming (SSE) usage extraction.
- Embedded, dated pricing table with per-model glob matching and a conservative
  high fallback for unknown models.
- End-to-end tests: a runaway `run` is killed at budget; `serve` refuses with 402
  once the rolling budget is crossed.

### Roadmap
- Trip notifications (webhook / desktop) and a one-line post-run spend summary.
- `--strict` pre-flight zero-overshoot mode.
- Request-dedup loop detection (identical-request hashing).
