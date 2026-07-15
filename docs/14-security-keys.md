# 14 — Security & API keys

## `breaker` never stores your API key

- The agent's API key rides on its requests (`x-api-key` for Anthropic,
  `Authorization: Bearer` for OpenAI). The proxy **forwards it untouched** to the
  upstream and does not persist or log it.
- For session grouping in `serve` mode, the key is **hashed** (`sha256`, first 12
  hex chars, prefixed `key:`) — the raw key is never used as a session id, stored,
  or shown on the dashboard.
- The `serve` journal (`--journal`) stores only spend events (timestamp, session
  id, model, cost) — **no keys, no prompts, no responses**.

## Trust boundary

- The proxy binds to `127.0.0.1` only. It is meant for local/CI use, not as a
  public gateway. Don't expose the port to untrusted networks — `/kill` and
  `/v1/*` have no auth of their own (they inherit the fact that only local
  processes can reach them).
- The dashboard's `/kill` is a plain POST; anyone who can reach the port can trip
  the breaker. That is intended for a single-operator local setup.

## What `breaker` can and cannot protect

It enforces spend on the **LLM calls it proxies**. It does not see or cap spend
that bypasses the proxy, nor cloud/serverless bills (see
[16 — roadmap & non-goals](16-roadmap-nongoals.md)).
