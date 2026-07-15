# 10 — Dashboard (serve mode)

`breaker serve` serves a one-page web UI on the same port as the proxy. The proxy
handles `/v1/*`; the dashboard handles everything else.

## Endpoints

| Path | Method | Purpose |
|---|---|---|
| `/` | GET | The dashboard page (embedded HTML/CSS/JS via `go:embed`). |
| `/api/state` | GET | JSON snapshot: window label, budget, spend, killed flag, per-session breakdown, recent activity. |
| `/kill` | POST | Manual trip — refuse all further requests. Returns 204. |

## UI

The page polls `/api/state` every second and renders a spend-vs-budget gauge
(green → amber > 80% → red when over/killed), the per-session breakdown, a recent
activity log (with an `est` tag on estimated events), and a **KILL** button that
POSTs `/kill`. It is theme-aware (light/dark) and dependency-free.

## Data source

The `dashboard.Controller` interface (`State()` + `Kill()`) is implemented by
serve's `windowGuard`, which builds the snapshot from the rolling store.
