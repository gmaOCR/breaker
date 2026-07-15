# 03 — Pricing

Cost = tokens × per-model price. The table lives in
`internal/pricing/prices.json`, embedded in the binary via `go:embed`.

## Table shape

```json
{ "version": "2026-07-15", "unit": "per_mtok",
  "models": { "claude-opus-4-[5-8]*": {"input": 5, "output": 25, "cache_write": 6.25, "cache_read": 0.5} },
  "fallback": {"input": 20, "output": 80, "cache_write": 25, "cache_read": 2} }
```

- Prices are **USD per million tokens**. Cost sums input + output + cache-write +
  cache-read.
- Keys are **globs** (`path.Match`): `claude-opus-4-[5-8]*` matches Opus 4.5–4.8
  and dated snapshots; exact matches win over globs.
- `version` is a date; `breaker version` prints it so staleness is visible.

## Unknown models fail conservative

If no model matches, the **high `fallback`** price is used, `matched=false` is
returned, the event is flagged `Estimated`, and the proxy logs one stderr warning
per unknown model. `Cost` never returns zero for real usage — a zero price would
mean the breaker never trips, which is the one bug that would defeat the product.

## Overriding prices

`--prices <file>` shallow-merges an override (per model) over the embedded table —
add self-hosted models or correct drift without rebuilding. Updating the shipped
table is a one-line JSON edit plus a release; no code change.
