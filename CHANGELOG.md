# Changelog

Changes that alter what a consuming project sees. The version is the release tag.

## Unreleased

### Added

- **`go-qa run --changed --also-packages <dir>,...`** runs the named packages and their importers
  as if they had changed, for a change the diff cannot see (a file outside the root that their tests
  read). An entry is a root-relative package directory or `<dir>/...`; one that names no package is
  a usage error. Named packages are tested (without `-race`) but not counted by coverage, and the
  scope line counts them.

- **Every step prints its start and end with its wall time.** `go-qa: test: start`, then
  `go-qa: test: passed in 41.2s`, on stderr as the run goes, and a `go-qa stage times:` table with
  the total at the end. A long CI run now shows where its time went.

- **The test step prints its 10 slowest packages and 10 slowest tests** to stderr after it has run,
  with their elapsed times, from `go test -json`. The tests' own output, and a failure's text, are
  rebuilt from the JSON and read as before.
- **Documented: what a diff run does.** `--changed` tests the changed packages and, transitively,
  every package of the module that imports one (`-race` on the changed ones only); a change to
  `go.mod`, `go.sum`, `go.work`, `tools.lock`, the golangci configuration, the project record, the
  go-qa configuration file or a configured rule source widens the run to every package. A
  `when = "full"` stage never runs when `--changed` is given, widened or not. `CI=true` alone does
  not select diff mode: without `--changed` a run is a full run and runs those stages.

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
