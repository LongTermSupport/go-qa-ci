# fmt and the golangci detector

Both tools run the `golangci-lint` binary, which must be on `PATH`; a missing binary fails the step
(`golangci-lint not found on PATH`), it is never skipped. The pinned release is in
`tools.lock`; `scripts/install-tools.sh golangci-lint` installs it after verifying the archive against
the sha256 pinned beside it in `tools.lock` (CI uses the same script). Tests that need the binary fail
when it is missing, like those of every other pinned tool (`GQACI_TEST_WITHOUT_TOOLS`, docs/astgrep.md).

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
  strict baseline `configs/golangci.yml` is used. The file is looked for in the module's own
  directory only: a nested module without a file of its own gets the baseline, even when a parent
  directory holds a `.golangci.yml` that golangci-lint's own discovery would pick. A project file's
  `linters.exclusions`, presets, `issues.new-from-*`, a check disabled inside a running linter and a
  narrowed `linters.default` can drop findings, so the `suppression` tool reads the file and requires a
  record entry for each (docs/suppression.md). Which linters run (`linters.enable`, `linters.disable`)
  is the defence set the project chooses, not a suppression: it needs no entry, and `go-qa rules`
  lists the linters as they are, so what a project does not run is visible rather than recorded; the settings of golangci-lint's own that it does not list are a recorded
  gap there.

## The bundled baseline

`configs/golangci.yml` is the golangci-lint v2 configuration for a project with none of its own:
the `standard` linters plus a stated set (bodyclose, errorlint, gosec, gocritic, revive, wrapcheck
and others), `errcheck` with type assertions and blank assignments checked, `govet` with every
analyzer except `fieldalignment`, `nolintlint` requiring a specific, explained, used directive, and
no cap on the number of issues. It carries no exclusion. `depguard` is not in the baseline because
its allow-list is specific to a project; this repository adds its own in the overlay below.

## Configuration exceptions

This repository's overlay configures away `gosec G204` (in the seven files that launch the tools and in test files) and `gosec G306` (one test
file). Each is listed in `docs/record.md` as a configuration exception, and the suppression
detector (docs/suppression.md) requires it to be recorded in `qa/record.toml`; so is the baseline's
disabling of the `govet` `fieldalignment` analyzer.

Contributors: the tests of `internal/cli`, `internal/golangci`, `internal/tools` and
`internal/defences` run the real `golangci-lint`, so `go test ./...` fails with
`golangci-lint not found on PATH` when it is absent. That is the "a missing tool is never a pass"
rule, not a flake: install the pinned binary with `scripts/install-tools.sh golangci-lint` and put
its directory on the search path before running the gates.

## Self-gate (TOOLING-SPEC 8.2)

CI runs `go-qa run --ci` over this repository (job `self-gate`). The configuration it analyses
go-qa with is derived from the bundled baseline, not kept separately:

- `configs/golangci.yml` is the baseline (embedded in the binary).
- `qa/golangci-overlay.yml` is the minimal overlay: the `depguard` allow-list and the two
  configuration exceptions above. The merge adds the overlay's keys to the baseline and does not
  override a value the baseline sets, but the baseline leaves `gosec.excludes` and
  `linters.exclusions.rules` unset, so an overlay could still weaken it through those keys; only the
  exact-content pin in `TestOverlayHoldsOnlyDocumentedExceptions` holds it to the documented
  exceptions.
- `.golangci.yml` is what golangci-lint discovers, so it is the merged result. golangci-lint cannot
  merge files, so it is mirrored by hand, and `tests/selfgate` fails unless it equals baseline plus
  overlay, carries every baseline linter, setting, formatter and issue cap, and the overlay holds
  nothing but the documented exceptions. The checker is itself tested against configurations that
  drop a linter, a formatter, a setting or the cap.

`qa/go-qa.toml` switches off only mutation testing, with the reason beside it.

Recorded gaps against 8.2 (TOOLING-SPEC 9.2): 8.2 is not met in full.

- Two Rules are not fully active on the toolchain's own source: `gosec::G204` (switched off in the
  seven files that launch the tools and in test files, a configuration exception; every other file is
  held to it) and `gosec::G306` (one test file). Both are in the record with
  a justification (docs/record.md), but the Rule is not held on this repository.
- Every Rule of a tool that is switched off in `qa/go-qa.toml` (mutation testing) is not active on the
  toolchain's own source until that tool is switched on.
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
- Bespoke rules (DETECTOR-SPEC 4.1) are served by the `analyzers` tool (docs/analyzers.md) and the `ast-grep` tool
  (docs/astgrep.md); nothing here adds a custom golangci-lint build.
