# 07 — Enforcement semantics

## When a request is refused

Before forwarding, the proxy asks the guard `Allowed()`. If not allowed, the
request never reaches the upstream and gets **HTTP 402** immediately (cheap — no
upstream call). The 402 body mirrors each provider's own error shape so SDKs treat
it as fatal rather than retrying:

- Anthropic: `{"type":"error","error":{"type":"breaker_budget_exceeded","message":"…"}}`
- OpenAI: `{"error":{"message":"…","type":"breaker_budget_exceeded","code":"budget_exceeded"}}`

`run` additionally kills the child on trip (see [06 — kill path](06-kill-path.md)),
so an agent that ignores the 402 is stopped anyway. `serve` keeps returning 402
for the whole time the rolling window is over budget.

## Post-response metering → one-response overshoot

Usage is only known once a response completes, so metering is **post-response**:
the request that crosses the budget is allowed to finish, and every *subsequent*
request is refused. This means spend can overshoot the cap by at most one response.

A `--strict` pre-flight worst-case mode (refuse before forwarding when
`spent + max_tokens × price > budget`) is on the roadmap for callers that cannot
tolerate any overshoot.
