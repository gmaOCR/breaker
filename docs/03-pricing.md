# 03. Pricing

Cost = tokens × per-model price. The table lives in
`internal/pricing/prices.json`, embedded in the binary via `go:embed`.

## Table shape

```json
{ "version": "2026-09-18", "unit": "per_mtok",
  "models": { "claude-opus-4-[5-8]*": {"input": 5, "output": 25, "cache_write": 6.25, "cache_read": 0.5} },
  "fallback": {"input": 20, "output": 80, "cache_write": 25, "cache_read": 2} }
```

- Prices are **USD per million tokens**. Cost sums input + output + cache-write +
  cache-read.
- Keys are **globs** (`path.Match`): `claude-opus-4-[5-8]*` matches Opus 4.5-4.8
  and dated snapshots. An **exact key wins over any glob**, which is how two
  models that share an input price but not a cache-read rate can coexist:
  `claude-fable-5-1` reads cache at 0.025x base while `claude-fable-5*` reads at
  0.1x. Globs are otherwise tried in sorted order, so matching is deterministic.
- `version` is a date; `breaker version` prints it so staleness is visible.

## Unknown models fail conservative

If no model matches, the **high `fallback`** price is used, `matched=false` is
returned, the event is flagged `Estimated`, and the proxy logs one stderr warning
per unknown model. `Cost` never returns zero for real usage: a zero price would
mean the breaker never trips, which is the one bug that would defeat the product.

## Overriding prices

`--prices <file>` shallow-merges an override (per model) over the embedded table,
so you can add self-hosted models or correct drift without rebuilding. Updating the shipped
table is a one-line JSON edit plus a release; no code change.

## Staying current, unattended

A table that goes stale mis-prices real spend, so `cmd/pricewatch` follows the
vendor's published prices on a schedule and releases the result without waiting
for anyone. Responsibilities are split three ways so the risky part is small and
testable:

| Piece | Job |
|---|---|
| `internal/pricing/upstream` | fetch the pricing page and parse its table. Columns are located by header text, never by position. Knows nothing about our table. |
| `pricing.Reconcile` | decide what may be applied unattended. All the safety rules live here, and all of them are tested offline against a saved copy of the page. |
| `cmd/pricewatch` | wire those two together, write the file, exit with a code the workflow reacts to. |

The rules exist because the risk is asymmetric: pricing a model too high only
trips the breaker early, while pricing it too low hands the user the bill they
installed `breaker` to avoid. So every rule fails closed.

- **Arithmetic integrity.** Anthropic documents cache writes at 1.25x the base
  input price and cache reads at 0.1x (0.025x on Fable 5.1 and Mythos 5.1). Each
  row's input price is therefore checked against two other independently parsed
  columns. A shifted or mis-read column cannot survive that, which is what makes
  applying a price *drop* without review defensible.
- **No silent gaps.** If a model the table prices no longer appears on the page,
  the whole update is refused. Keeping a stale price quietly is wrong; deleting
  the entry is worse.
- **No ambiguity.** If one glob now covers models priced differently, the update
  is refused and the pattern has to be split by hand.
- **New models are added under their exact id**, never as a new glob, so an
  unattended addition can never make an existing pattern ambiguous.
- **Non-Claude entries and the fallback are never touched.** OpenAI publishes
  prices tiered by context length, which this table cannot express, so those
  models deliberately stay on the conservative fallback.

A refusal opens an issue rather than guessing. Anything applied is committed,
tagged as a patch release, and published in one run: see
[15. release & versioning](15-release-versioning.md).

To check by hand, including offline against the saved page:

```console
go run ./cmd/pricewatch                                    # report only
go run ./cmd/pricewatch -write                             # apply
go run ./cmd/pricewatch -url internal/pricing/upstream/testdata/anthropic-pricing.md
```
