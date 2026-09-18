# 15. Release & versioning

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

Prices drift, so this is automated end to end. `.github/workflows/pricewatch.yml`
runs weekly: it reconciles `internal/pricing/prices.json` with the vendor's
published prices, and when anything moved it runs the suite, commits the table,
derives the next patch version from the latest tag, tags it, and publishes the
release. No click anywhere. The rules that make that safe are in
[03. pricing](03-pricing.md); when any of them refuses, the job opens an issue
and changes nothing.

Note the plumbing: the publish step is *called* from the pricewatch workflow
rather than left to the tag push, because a tag created with `GITHUB_TOKEN` does
not trigger workflows. A cascade would look right and never fire.

To do it by hand anyway, `go run ./cmd/pricewatch -write` then tag as above.
Users can also override without upgrading at all, via `--prices`.

## Changelog

Keep `CHANGELOG.md` current (Keep a Changelog format). Move items from `Roadmap`
to `Added` as they ship.
