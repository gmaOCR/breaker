# 15 — Release & versioning

## Versioning

SemVer. The binary version is stamped at build time via ldflags:

```
go build -ldflags "-X main.version=$(git describe --tags --always --dirty)" -o bin/breaker ./cmd/breaker
```

(`make build` does this.) `breaker version` prints the version and the embedded
pricing-table date.

## Cutting a release

```console
make test            # unit + e2e must pass
make xbuild          # cross-compile static binaries into dist/
git tag v0.1.0
git push origin v0.1.0
```

`make xbuild` produces `dist/breaker-<os>-<arch>` for linux/amd64, linux/arm64,
darwin/amd64, darwin/arm64, and windows/amd64 (CGO disabled, `-s -w`).

## Updating prices

Prices drift. Bump `internal/pricing/prices.json` (`version` = the date) and cut a
patch release. No code change is required — the table is embedded and glob-matched.
Users can also override without upgrading via `--prices`.

## Changelog

Keep `CHANGELOG.md` current (Keep a Changelog format). Move items from `Roadmap`
to `Added` as they ship.
