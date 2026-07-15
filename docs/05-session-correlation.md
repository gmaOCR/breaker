# 05 — Session correlation

Every metered event is attributed to a session, so budgets and the dashboard can
break spend down. The proxy resolves the session per request (`sessionFor`).

## run mode — single session

`breaker run` starts one proxy instance for one run, so **all** traffic is one
session (a random id). No cooperation from the agent is needed — this is why `run`
is drop-in.

## serve mode — multi-tenant

`breaker serve` handles many clients. The session is resolved in priority order:

1. `X-Breaker-Session` request header, if the client sets one;
2. else `key:<sha256(api-key)[:12]>` — grouped by API key, **hashed**; the raw key
   is never stored or logged;
3. else the remote address.

The rolling-window budget sums across **all** sessions; per-session totals exist
only for the dashboard breakdown.
