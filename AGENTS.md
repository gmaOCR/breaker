# AGENTS.md

Guidance for AI agents (and humans) working on **breaker** — a cost
circuit-breaker for AI agents, written in Go. (`AGENTS.md` is the cross-tool
convention; Claude Code, Cursor, Codex and others read it.)

## Commands

| Task | Command |
|------|---------|
| Build | `make build` → `./bin/breaker` |
| Test (unit + e2e) | `make test` |
| End-to-end only | `make e2e` |
| Format check | `gofmt -l .` (must print nothing) |
| Vet | `go vet ./...` |
| Cross-compile | `make xbuild` |

**Definition of done:** `gofmt` clean, `go vet ./...` clean, `go test ./...`
green. Any non-trivial logic ships a test.

## Project shape

Single Go binary, **standard library only** — no third-party dependencies; keep
it that way.

- `cmd/breaker` — CLI (`run`, `serve`, `version`)
- `internal/{core,pricing,metering,proxy,breaker,policy,store,runner,dashboard,notify}`
- Docs: `docs/00-overview.md` → `docs/16`; architecture in `docs/01-architecture.md`.

Design seam: the proxy depends on the small `proxy.Guard` interface, not a
concrete engine — `run` plugs in the one-shot `breaker.Engine`, `serve` plugs in
a rolling-window guard. Add features behind that seam.

## Go conventions

- Format with `gofmt`; group imports stdlib / third-party / local via `goimports`.
- Names: `MixedCaps`, short receivers, no `Get` prefix, acronyms in constant case
  (`ID`, `URL`). Document exported identifiers with a comment starting with the name.
- Errors: return them; wrap with `fmt.Errorf("…: %w", err)`; inspect with
  `errors.Is`/`errors.As`. No `panic` in library code. Don't ignore a meaningful error.
- Prefer `any` over `interface{}`. Use `log/slog` if structured logging is needed.
- Concurrency: guard shared state with a mutex (see `breaker.Engine`, `store.Store`);
  document what a lock covers; run `go test -race` on concurrency changes.
- Tests: table-driven, `t.Run` subtests, `t.Parallel()` when safe, `t.TempDir()`,
  `httptest` — stdlib `testing` only, no frameworks.
- Stay lazy: no unrequested abstractions, no dependency for what stdlib does in a
  few lines, shortest change that works. Mark deliberate shortcuts with a
  `// ponytail:` comment naming the ceiling.

## Product invariants (do not break)

- Pricing must **never** return zero cost for real usage — a zero price means the
  breaker never trips. Unknown models use the high fallback, flagged `estimated`.
- No silent-zero metering: if a provider reports no usage, estimate from size and
  flag `estimated`.
- Honesty boundary: breaker enforces **agent LLM spend only** — never claim to cap
  cloud/serverless bills (`docs/16-roadmap-nongoals.md`).

## Commits

`[TAG] scope: Title` — `[ADD]` / `[FIX]` / `[REF]` / `[IMP]` / `[RM]`. Push to
feature branches; never force-push `main`.

## Go references (authoritative, community-proven)

- Effective Go — <https://go.dev/doc/effective_go>
- Go Code Review Comments — <https://go.dev/wiki/CodeReviewComments>
- Google Go Style Guide — <https://google.github.io/styleguide/go/>
- Uber Go Style Guide — <https://github.com/uber-go/guide>
- Tools: golangci-lint <https://golangci-lint.run>, staticcheck <https://staticcheck.dev>, govulncheck <https://go.dev/blog/govulncheck>
