# 00 — Overview

`breaker` is a **cost circuit-breaker for AI agents**: it puts a hard dollar (or
token) budget on an agent run and terminates the run before the budget is
exceeded.

## The problem

AI coding agents consume 10–100× the tokens of a chatbot. A looping agent, a
runaway retry, or an unattended overnight run can produce a very large bill with
no human in the loop. Provider "spend limits" are **alerts**: evaluated
periodically and delivered after the spend has already happened. There is no
provider-side primitive that hard-stops an in-flight run at a threshold.

## The approach

`breaker` inserts itself as a local reverse proxy between the agent and the LLM
API. Because the agent's traffic flows through it, `breaker` can:

1. **Meter** real token usage from each response.
2. **Price** that usage with an embedded, dated table.
3. **Enforce** the budget: once cumulative spend crosses the cap, further API
   calls are refused (HTTP 402) and the wrapped process is killed.

This is the honest scope of the tool: it enforces spend on the **LLM calls of an
agent whose process it controls**. It does not attempt to cap cloud/serverless
provider bills — that would require an enforcement primitive the cloud providers
do not expose (see [16 — roadmap & non-goals](16-roadmap-nongoals.md)).

## Reading order

- [01 — architecture](01-architecture.md) — proxy, metering, engine, runner.
- [09 — CLI reference](09-cli-reference.md) — commands and flags.
- [12 — verification](12-verification.md) — how to prove it kills a run.
- [16 — roadmap & non-goals](16-roadmap-nongoals.md) — what's built, what's next,
  and the honesty boundary.
