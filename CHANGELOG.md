# Changelog

All notable changes to this project are documented here. Format loosely follows
[Keep a Changelog](https://keepachangelog.com/); versions follow SemVer.

## [Unreleased]

### Added
- **Automated price tracking, with no click anywhere.**
  `.github/workflows/pricewatch.yml` runs weekly, reconciles the embedded table
  with the vendor's published prices, and when anything moved it runs the suite,
  commits, derives the next patch version, tags it and publishes the release.
  Responsibilities are split three ways: `internal/pricing/upstream` only
  fetches and parses, `pricing.Reconcile` holds every rule about what may be
  applied unattended, and `cmd/pricewatch` only wires them together.
- The rules fail closed, because pricing a model too low hands the user the bill
  breaker exists to prevent. Each row's input price is verified against the
  documented cache multipliers (1.25x write, 0.1x read, 0.025x on Fable 5.1), so
  a shifted column cannot pass as a plausible price. An entry that vanishes from
  the page, or a glob that now covers models priced differently, refuses the
  whole update and opens an issue. New models are added under their exact id,
  never as a new glob. Non-Claude entries and the fallback are never touched.
- `claude-mythos-5` and `claude-mythos-5-1`, found by the new tooling on its
  first run against the live page.

### Changed
- Glob matching in `Table.lookup` now iterates in sorted order, so two
  overlapping patterns resolve deterministically instead of by map order. This
  also lets the reconciler prove it writes to the key the proxy actually reads.
- `release.yml` gained a `workflow_call` entry point, because a tag created with
  `GITHUB_TOKEN` does not trigger a workflow: the pricewatch job has to call the
  publish step rather than rely on the tag push.
- Every em dash in the repo is gone, including in four stderr strings.

## [0.2.0] - 2026-09-17

### Fixed
- **Pricing: `claude-opus-5` matched no pattern.** `claude-opus-4-[5-8]*` requires
  the literal prefix `claude-opus-4-`, so every Opus 5 run priced at the $20/$80
  fallback and was flagged `estimated`: the breaker tripped roughly four times too
  early and reported a spend that was plainly wrong. Added `claude-opus-5*`.
- **Pricing: `claude-sonnet-5` was billed at Sonnet 4.6 rates** ($3/$15 instead of
  $2/$10), a 50% overcharge. Sonnet 5's introductory $2/$10 is now its standard
  price.
- **Pricing: Claude Fable 5.1 reads cache at 0.025x base**, not the usual 0.1x, so
  it needs its own entry. An exact key beats the `claude-fable-5*` glob in
  `Table.lookup`, which keeps the two disjoint.
- `make e2e` ran only `TestBreakerKillsRunOverBudget`; its `-run` filter silently
  skipped `TestServeRollingBudgetRefuses`, the only end-to-end proof that `serve`
  refuses with 402 once the rolling window is spent.
- `breaker` with no arguments printed a hand-maintained flag list that had drifted,
  missing `--port`, all three velocity and loop guards, and both notification
  flags, plus every `serve` flag. It now points at `breaker run -h` / `serve -h`,
  which the `flag` package generates from the real flag sets.
- `--anthropic-upstream` / `--openai-upstream` reported an empty default in `-h`
  because the real value was only applied downstream in `proxy.New`. Both now
  default to the exported `proxy.Default*Upstream` constants.

### Added
- `.github/workflows/release.yml`: pushing a `v*` tag runs the suite,
  cross-compiles, verifies the binaries carry the tag's version, and publishes a
  GitHub Release with all five artefacts.
- Pricing regression tests: every model actually run must match a pattern rather
  than fall through to the fallback, and Fable 5.1's cache-read rate is pinned.

### Documentation
- `docs/01-architecture.md` listed `notify` as roadmap although it ships, and
  omitted the `Dedup` policy.
- `docs/09-cli-reference.md` claimed `--daily` and `--hourly` were mutually
  exclusive while the code lets `--hourly` win; it also documented only exit code
  `137`, never `1` and `2`.
- `docs/12-verification.md` quoted a trip message missing its
  `($X spent) [policy]` suffix, never mentioned the `serve` end-to-end test, and
  costed a mock call with the superseded Sonnet price.
- `README.md` named two injected environment variables out of four.
- `docs/15-release-versioning.md` walked through tagging `v0.1.0`, already
  released, and predated the release workflow.

### Security
- Built with Go 1.27.1 (was 1.24.13), closing 11 reachable Go standard-library
  advisories (`crypto/tls`, `crypto/x509`, `net/http`, `net/url`,
  `net/textproto`, `os`) reported by `govulncheck`.

### Changed
- `go.mod` now carries a single `go 1.27.1` directive instead of a `go` /
  `toolchain` pair. `actions/setup-go` reads only the `go` directive, so the
  split made CI install one Go and `GOTOOLCHAIN` switch to another, which in
  turn built `govulncheck` against an older stdlib than the one it had to scan.

## [0.1.0] - 2026-07-15

### Added
- `breaker run`: hard per-run budget (`--budget` USD and/or `--tokens`); the
  wrapped process is SIGKILLed when the cap trips.
- Velocity guard (`--max-per-min`, `--max-calls-per-min`) and loop guard
  (`--max-repeats`, by request-body fingerprint) that trip *before* the absolute
  cap on a spend/call-rate spike or identical-request repetition.
- `breaker serve`: long-lived proxy enforcing a rolling per-window budget
  (`--daily` / `--hourly`) shared across runs, with an append-only JSONL journal
  (`--journal`) so the window survives restarts.
- One-page embedded web dashboard (live gauge, per-session breakdown, activity
  log, manual KILL button) served by `breaker serve`.
- Trip notifications: `--notify-webhook` (JSON POST) and `--notify-desktop`, on
  both `run` and `serve`, plus a one-line post-run spend summary.
- Metering reverse proxy for the Anthropic Messages API and OpenAI-compatible
  Chat Completions (streaming + non-streaming), with a size-based usage estimator
  (flagged `estimated`) when a provider reports none; never a silent zero.
- Embedded, dated pricing table with per-model glob matching and a conservative
  high fallback for unknown models (with a one-time stderr warning).
- Test suite across all logic packages (unit) plus end-to-end proofs: a runaway
  `run` is killed at budget, and `serve` refuses with 402 over the rolling budget.

### Roadmap
- `--strict` pre-flight zero-overshoot mode.
