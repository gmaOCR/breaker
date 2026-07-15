# Changelog

All notable changes to this project are documented here. Format loosely follows
[Keep a Changelog](https://keepachangelog.com/); versions follow SemVer.

## [0.1.0] — 2026-07-15

### Added
- `breaker run` — hard per-run budget (`--budget` USD and/or `--tokens`); the
  wrapped process is SIGKILLed when the cap trips.
- Velocity guard (`--max-per-min`, `--max-calls-per-min`) and loop guard
  (`--max-repeats`, by request-body fingerprint) that trip *before* the absolute
  cap on a spend/call-rate spike or identical-request repetition.
- `breaker serve` — long-lived proxy enforcing a rolling per-window budget
  (`--daily` / `--hourly`) shared across runs, with an append-only JSONL journal
  (`--journal`) so the window survives restarts.
- One-page embedded web dashboard (live gauge, per-session breakdown, activity
  log, manual KILL button) served by `breaker serve`.
- Trip notifications — `--notify-webhook` (JSON POST) and `--notify-desktop`, on
  both `run` and `serve` — plus a one-line post-run spend summary.
- Metering reverse proxy for the Anthropic Messages API and OpenAI-compatible
  Chat Completions (streaming + non-streaming), with a size-based usage estimator
  (flagged `estimated`) when a provider reports none — never a silent zero.
- Embedded, dated pricing table with per-model glob matching and a conservative
  high fallback for unknown models (with a one-time stderr warning).
- Test suite across all logic packages (unit) plus end-to-end proofs: a runaway
  `run` is killed at budget, and `serve` refuses with 402 over the rolling budget.

### Roadmap
- `--strict` pre-flight zero-overshoot mode.
