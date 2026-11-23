# Changelog

Changes that alter what a consuming project sees. The version is the release tag.

## Unreleased

### Changed

- **The test step uses Go's test cache in local runs.** `go-qa run` without `--ci` runs
  `go test -race` (with the coverage flags), so a test whose inputs did not change is answered from
  the cache. It ran `go test -race -shuffle=on -count=1`, which reran every test on every run.
- **Random order only in forced runs.** go never caches a run with `-shuffle`, so shuffling now
  happens only in a forced run: every CI run (`--ci`, or a `CI` variable that is set and not `false`
  or `0`), `--no-cache`, or `test.cache = false`. These add `-shuffle=on -count=1`. A local run is in
  source order.
- **`--changed` runs `-race` on the changed packages only.** Their importers' tests run without it.
  A race that only an importer's tests exercise is missed by a `--changed` run, and is caught by the
  full run and by every CI run.
- **A cached run refuses a `GOFLAGS` holding `-count` or `-shuffle`** and names the flag.

### Added

- `[test] cache` (default `true`) and `go-qa run --no-cache`.
- `[[stage]] when = "full"`: the stage runs in full runs only, never in a `--changed` run.
- A stage running `go test` with `-count` or `-shuffle` (directly, after `go -C <dir>`, or in
  `GOFLAGS` behind `env`) is a configuration error unless it says `when = "full"`.
