# fmt and the golangci detector

Both tools run the `golangci-lint` binary, which must be on `PATH`; a missing binary fails the step
(`golangci-lint not found on PATH`), it is never skipped. CI installs the pinned release with
`.github/scripts/install-golangci-lint.sh`, which verifies the archive against the checksum file
published with the release.

## `fmt`

Applies the formatters the golangci configuration enables (`gofumpt`, `goimports` in the bundled
baseline) with `golangci-lint fmt`.

| Mode                 | Behaviour                                                                                                       |
| -------------------- | --------------------------------------------------------------------------------------------------------------- |
| default              | rewrites the files; passes                                                                                      |
| `--no-fix`           | rewrites nothing; fails and prints the diff as `error[fmt]`                                                     |
| `--ci` (or `CI` set) | rewrites, then fails with `working tree changed` and the files rewritten, so CI cannot pass on unformatted code |

"Working tree changed" is judged by comparing content hashes of every Go file before and after
formatting, so it does not depend on git, or on the tree being clean when the run starts. `-p`
limits formatting to the named files and directories.

## `golangci`

Runs `golangci-lint run` over `./...` of the root module and of each `modules` entry, reads the
JSON report and prints each finding compiler-style, identifier first:

```text
error[errcheck]: Error return value of `os.Remove` is not checked
  --> p/p.go:8
error[staticcheck::SA1019]: "io/ioutil" has been deprecated since Go 1.19: ...
  --> q/q.go:4
```

Identifiers are printed as the tools emit them (TOOLING-SPEC 4.4): the linter name, and for
`staticcheck`, `gosec`, `govet`, `gocritic` and `revive`, whose messages start with the check code,
`linter::code`.

- Scope: `-p` filters the findings after the whole module has been analysed, because a linter that
  sees only some files loses the type information of the rest. Findings in files outside every `-p`
  entry are dropped.
- Record: an exception with `covers = "findings"` whose rule and path match a finding licenses it.
  An exception that covers suppressions never licenses a finding. Staleness of unused
  `covers = "findings"` exceptions is not judged here (it needs every detector that applies the
  record); the `suppression` tool judges the suppression ones.
- Configuration: a project `.golangci.{yml,yaml,toml,json}` is used as it is. Without one, the
  strict baseline `configs/golangci.yml` is used.

## The bundled baseline

`configs/golangci.yml` is the golangci-lint v2 configuration for a project with none of its own:
the `standard` linters plus a stated set (bodyclose, errorlint, gosec, gocritic, revive, wrapcheck
and others), `errcheck` with type assertions and blank assignments checked, `govet` with every
analyzer except `fieldalignment`, `nolintlint` requiring a specific, explained, used directive, and
no cap on the number of issues. It carries no exclusion. `depguard` is not in the baseline because
its allow-list is specific to a project; this repository adds its own in the overlay below.

## Configuration exceptions

This repository's overlay configures away `gosec G204` (module-wide) and `gosec G306` (one test
file). Each is listed in `docs/record.md` as a configuration exception, and the suppression
detector (docs/suppression.md) requires it to be recorded in `qa/record.toml`; so is the baseline's
disabling of the `govet` `fieldalignment` analyzer.

## Self-gate (TOOLING-SPEC 8.2)

CI runs `go-qa run --ci` over this repository (job `self-gate`). The configuration it analyses
go-qa with is derived from the bundled baseline, not kept separately:

- `configs/golangci.yml` is the baseline (embedded in the binary).
- `qa/golangci-overlay.yml` is the minimal overlay: the `depguard` allow-list and the two
  configuration exceptions above. An overlay can add to the baseline but not change a value it sets.
- `.golangci.yml` is what golangci-lint discovers, so it is the merged result. golangci-lint cannot
  merge files, so it is mirrored by hand, and `tests/selfgate` fails unless it equals baseline plus
  overlay, carries every baseline linter, setting, formatter and issue cap, and the overlay holds
  nothing but the documented exceptions. The checker is itself tested against configurations that
  drop a linter, a formatter, a setting or the cap.

`qa/go-qa.toml` switches off the tools that are not built yet so that none is reported as passing
without running.

Recorded gaps against 8.2 (TOOLING-SPEC 9.2): 8.2 is not met in full.

- Every Rule of a tool that is switched off in `qa/go-qa.toml` is not active on the toolchain's own
  source until that tool is built and switched on.
- The merged `.golangci.yml` is kept in step by a failing test, not generated, so an edit to the
  baseline fails the build until the file is mirrored; it cannot silently drift.
- Only the golangci-lint rules are proven active by `tests/selfgate`: the configuration carries
  every baseline defence, `qa/go-qa.toml` leaves `fmt` and `golangci` on, and the CI self-gate job
  runs `go-qa run --ci`. The other bundled tools have no such test yet. The proof does not execute
  the workflow itself: a hosted run that never happens is outside what a test can see.

## Recorded gaps (TOOLING-SPEC 9.2)

go-qa does not claim conformance; the golangci-lint and staticcheck register pages grade these
clauses of DETECTOR-SPEC, and the gaps stand:

- 4.3 partial: golangci-lint prints one name per linter, and wrapped tools' codes pass through. go-qa
  prints `linter::code` for the five linters above only; the other linters that wrap several checks
  print the linter name alone.
- 6.1 to 6.3 partial: a printed golangci-lint or staticcheck identifier does not resolve through
  `go-qa rule-doc`; it resolves only through `golangci-lint help linters` or `staticcheck -explain`
  run by hand, or the tool's website.
- 4.4: there is no rule over the rules in golangci-lint; the identifier discipline for third-party
  linters is not enforced by go-qa yet.
- 6.4 not verified: no release gate over third-party documentation.
- staticcheck 7.1 (`//lint:ignore` cannot be disabled): the inline route is detected, not disabled.
  The `suppression` tool fails every `//lint:ignore`, `//lint:file-ignore` and `staticcheck.conf`
  removal that the record does not license; `nolintlint` covers `//nolint` only.
- Bespoke rules (DETECTOR-SPEC 4.1) are served by the `analyzers` tool and the `ast-grep` tool, which
  are not built yet; nothing here adds a custom golangci-lint build.
