# 12 — Verification

## Automated

```console
make test    # unit tests: pricing, metering (SSE), engine trip logic
make e2e     # end-to-end: proves breaker SIGKILLs a run at budget
```

The end-to-end test (`test/e2e_test.go`) builds the binary, a fake Anthropic
upstream (`test/mockllm`, fixed 1000-in/1000-out per call ≈ $0.018 on Sonnet-5),
and a loop client that hammers the proxy forever and ignores HTTP errors — so the
*only* thing that can stop it is the breaker's kill. The test asserts:

- exit code `137` (killed),
- `TRIPPED` in the breaker's output,
- within a 15s deadline.

It is fully offline and needs no API key.

## Manual smoke test (real API)

With `ANTHROPIC_API_KEY` set, cap a real Claude Code run very low and watch it
trip:

```console
breaker run --budget 0.05 -- claude -p "count from 1 to 100000, one number per message"
```

Expected: the run stops with a `breaker: TRIPPED — budget of $0.05 reached` line
and a non-zero (`137`) exit code, having spent just over $0.05.

## Convention

Every bug fix ships a non-regression test named after the bug (per the project's
development guidelines).
