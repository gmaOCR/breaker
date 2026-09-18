# 12. Verification

## Automated

```console
make test    # everything: unit tests plus both end-to-end tests
make e2e     # the two end-to-end tests only, verbose
```

The end-to-end test (`test/e2e_test.go`) builds the binary, a fake Anthropic
upstream (`test/mockllm`, fixed 1000-in/1000-out per call = $0.012 on Sonnet 5),
and a loop client that hammers the proxy forever and ignores HTTP errors, so the
*only* thing that can stop it is the breaker's kill. The test asserts:

- exit code `137` (killed),
- `TRIPPED` in the breaker's output,
- within a 15s deadline.

It is fully offline and needs no API key.

A second end-to-end test (`test/serve_test.go`) covers `serve` the same way: it
starts a real proxy with `--daily 0.05` against the same fake upstream, drives it
until the rolling window is over budget, and asserts that further requests get a
`402` and that the dashboard's manual KILL endpoint works. `make e2e` runs both.

## Manual smoke test (real API)

With `ANTHROPIC_API_KEY` set, cap a real Claude Code run very low and watch it
trip:

```console
breaker run --budget 0.05 -- claude -p "count from 1 to 100000, one number per message"
```

Expected: the run stops with a line of the form
`breaker: TRIPPED, budget of $0.05 reached ($0.0600 spent) [hardcap]` and a
non-zero (`137`) exit code, having spent just over $0.05. The trailing
`[<policy>]` names the policy that fired: `hardcap`, `velocity` or `dedup`.

## Convention

Every bug fix ships a non-regression test named after the bug (per the project's
development guidelines).
