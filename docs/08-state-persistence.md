# 08 — State & persistence

## run mode — in-memory

`breaker run` holds spend in the `breaker.Engine` (a mutex-guarded counter). The
process *is* the run; there is nothing to persist. When it exits, state is gone.

## serve mode — rolling window + journal

`breaker serve` needs its rolling budget to survive restarts, so it uses
`internal/store`:

- Every metered event is appended to an in-memory slice and (if `--journal <path>`
  is set) written as one JSON line to an append-only journal.
- `WindowSum(d)` / `SessionSums(d)` sum events newer than `now-d`.
- `Add` prunes events older than the store's `keep` (the budget window), so memory
  stays bounded to the window.
- On startup, `Open` replays the journal and keeps only events still inside the
  window.

`ponytail:` the journal is append-only and only compacted on startup — it grows
within a single long-lived process. Add size-triggered rewrite if a `serve`
process is meant to run for months. A pure-Go SQLite backend is the upgrade path
if you later want historical analytics beyond the window.
