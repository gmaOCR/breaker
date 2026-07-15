# 04 — Policies (run mode)

In `run` mode the `breaker.Engine` evaluates a list of policies after every
metered event; the first that trips fires a one-shot trip (which kills the child).
Policies implement `policy.Policy` — `Name()` and `Check(State, SpendEvent)`.

| Policy | Trips when | Enabled by |
|---|---|---|
| `HardCap` | cumulative spend ≥ `--budget` (USD) or tokens ≥ `--tokens` | always (the baseline) |
| `Velocity` | spend $/min > `--max-per-min`, or calls/min > `--max-calls-per-min`, over a rolling 60s window | either flag set |
| `Dedup` | the same request (by body fingerprint) repeats more than `--max-repeats` times in a rolling minute | `--max-repeats` set |

- **HardCap** is the guaranteed ceiling. **Velocity** and **Dedup** trip *earlier*,
  catching runaway loops before they reach the absolute cap — Velocity by rate,
  Dedup by identical-request repetition (`SpendEvent.ReqHash`, a sha256 of the
  request body computed in the proxy).
- The trip is **one-shot**: once any policy trips, the run is killed and later
  events don't re-trip.
- `serve` mode does not use this engine — it enforces a rolling-window budget
  directly (see [08 — state](08-state-persistence.md)).
