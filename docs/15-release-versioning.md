# 15 — Release & versioning

## Versioning

SemVer. The binary version is stamped at build time via ldflags:

```
go build -ldflags "-X main.version=$(git describe --tags --always --dirty)" -o bin/breaker ./cmd/breaker
```

(`make build` does this.) `breaker version` prints the version and the embedded
pricing-table date.

## Cutting a release

Pushing a `v*` tag is the whole release. `.github/workflows/release.yml` runs the
full test suite, cross-compiles, and publishes a GitHub Release with the five
binaries attached:

```console
# 1. move CHANGELOG.md's [Unreleased] section under the new version heading
# 2. tag and push; the workflow does the rest
git tag v0.2.0
git push origin v0.2.0
```

Build locally first if you want to inspect the artefacts by hand:

```console
make test            # unit + e2e must pass
make xbuild          # cross-compile static binaries into dist/
```

`make xbuild` produces `dist/breaker-<os>-<arch>` for linux/amd64, linux/arm64,
darwin/amd64, darwin/arm64, and windows/amd64 (CGO disabled, `-s -w`). The release
workflow runs exactly these two targets, so a green `make test` locally means a
green release.

## Updating prices

Prices drift. Bump `internal/pricing/prices.json` (`version` = the date) and cut a
patch release. No code change is required — the table is embedded and glob-matched.
Users can also override without upgrading via `--prices`.

## Changelog

Keep `CHANGELOG.md` current (Keep a Changelog format). Move items from `Roadmap`
to `Added` as they ship.
