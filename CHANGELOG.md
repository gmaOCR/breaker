# Changelog

All notable changes to this project are documented here. Format loosely follows
[Keep a Changelog](https://keepachangelog.com/); versions follow SemVer.

## [Unreleased]

### Added
- `breaker run` — wrap a command with a hard per-run budget (`--budget` USD and/or
  `--tokens`); the wrapped process is SIGKILLed when the cap trips.
- Metering reverse proxy for the Anthropic Messages API and OpenAI-compatible
  Chat Completions, including streaming (SSE) usage extraction.
- Embedded, dated pricing table (`internal/pricing/prices.json`) with per-model
  glob matching and a conservative high fallback for unknown models.
- End-to-end test proving a runaway run is killed at budget.

### Roadmap
- Velocity / loop guard (trip before the absolute cap).
- `breaker serve` — standalone proxy + rolling per-window budget across runs.
- One-page local web dashboard with a manual KILL button.
- Trip notifications (webhook / desktop) and `--strict` zero-overshoot mode.
