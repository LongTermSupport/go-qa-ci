# fmt and the golangci detector

Both tools run the pinned `golangci-lint` binary. `go-qa run` installs the release `tools.lock` pins
(`go-qa install-tools` does it ahead of time) after verifying the archive against the sha256 pinned
beside it, and runs that copy; nothing is put on `PATH` by hand. A binary that is still missing
fails the step (`golangci-lint not found on PATH`), it is never skipped. Tests that need the binary fail
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
  exclusion paths and presets, `issues.new-from-*`, a `*.disable-all` and a narrowed
  `linters.default` can drop findings, so the `suppression` tool reads the file and requires a
  record entry for each; a `linters.exclusions.rules` entry fails as
  `gqaci.suppression.exclusion-rule`, which no record entry can license (docs/suppression.md).
  Which checks run is the defence set the project chooses, not a suppression: a linter in
  `linters.disable`, or one rule in its linter's own switch-off list (`gosec.excludes`,
  `govet.disable`, a staticcheck `-XXXX` check and the like), needs no entry, only a one-line reason
  beside it; `go-qa rules` lists the linters as they are, so what a project does not run is visible
  rather than recorded; the settings of golangci-lint's own that it does not list are a recorded
  gap there.

## The bundled baseline

`configs/golangci.yml` is the golangci-lint v2 configuration for a project with none of its own.
It follows Go's own opinions: a check runs on all code or is off, and a check that is more noise
than signal is off. Each linter it enables carries the reason on its line.

- `linters.default: standard` (errcheck, govet, ineffassign, staticcheck, unused), and no
  `enable-all` anywhere.
- Linters at least one major Go project enables: errorlint, bodyclose, exhaustive, misspell,
  unconvert, usestdlibvars, predeclared, testifylint, nolintlint, unparam, gocritic (default checks
  only), revive (default rules only), gosec.
- Narrow bug patterns: nilerr, nilnil, errchkjson, durationcheck, thelper.
- Complexity: gocognit, the only measure.
- `govet`: vet's defaults plus the analyzers gopls runs by default (nilness, unusedwrite,
  atomicalign, deepequalerrors, sortslice, timeformat); shadow and fieldalignment stay off.
- `errcheck` with type assertions and blank assignments checked, `nolintlint` requiring a
  specific, explained, used directive, and no cap on the number of issues.

Off, because they are noisier than they are reliable or enforce a style Go does not ask for:
wrapcheck, depguard, forcetypeassert, errname, noctx, contextcheck, tparallel, rowserrcheck,
sqlclosecheck, dupword, goconst, exhaustruct, varnamelen, wsl, ireturn, err113,
gochecknoglobals, paralleltest, testpackage, lll, mnd, godot, cyclop, gocyclo, funlen, nestif. A
project that wants one enables it in its own configuration (`depguard` needs an allow-list specific
to a project; this repository adds its own in the overlay below). The baseline carries no
exclusion. `configs/golangci_test.go` holds the baseline to exactly this set.

## Switch-offs

This repository's overlay switches off one check for all code: gosec `G204` (a subprocess launched
with a variable program or arguments), in `linters.settings.gosec.excludes`, with its reason on
the line. go-qa is a tool runner: starting programs from an argument vector is what it does, so
the rule would flag its purpose rather than a defect, and the hazard it guards against (command
injection through a shell) is held by go-qa's own analyzer `gqaci_shell_exec`, which forbids
starting a shell with `-c`. Package `internal/launch` remains the one place a process is started,
with an argument vector only, a program resolved on PATH or given as an absolute path, and a
context bound to the process. A switch-off for all code needs no record entry
(docs/suppression.md), so `qa/record.toml` holds none.

Contributors: the tests of `internal/cli`, `internal/golangci`, `internal/tools` and
`internal/defences` run the real `golangci-lint`, so `go test ./...` fails with
`golangci-lint not found on PATH` when it is absent. That is the "a missing tool is never a pass"
rule, not a flake: install the pinned binary with `scripts/install-tools.sh golangci-lint` and put
the directory `scripts/install-tools.sh --print-path golangci-lint` prints on the search path
before running `go test`. (`go-qa run` does this itself for the tests it starts.)

## Self-gate (TOOLING-SPEC 8.2)

CI runs `go-qa run --ci` over this repository (job `self-gate`). The configuration it analyses
go-qa with is derived from the bundled baseline, not kept separately:

- `configs/golangci.yml` is the baseline (embedded in the binary).
- `qa/golangci-overlay.yml` is the minimal overlay: the `depguard` allow-list and the
  switch-off above. The merge adds the overlay's keys to the baseline and does not override a
  value the baseline sets, but the baseline leaves `gosec.excludes` and `linters.exclusions`
  unset, so an overlay could still weaken it through those keys; only the exact-content pin in
  `TestOverlayHoldsOnlyDocumentedSwitchOffs` holds it to the documented switch-off, and the
  suppression detector fails any exclusion rule.
- `.golangci.yml` is what golangci-lint discovers, so it is the merged result. golangci-lint cannot
  merge files, so it is mirrored by hand, and `tests/selfgate` fails unless it equals baseline plus
  overlay, carries every baseline linter, setting, formatter and issue cap, and the overlay holds
  nothing but the documented exceptions. The checker is itself tested against configurations that
  drop a linter, a formatter, a setting or the cap.

`qa/go-qa.toml` switches off only mutation testing, with the reason beside it.

Recorded gaps against 8.2 (TOOLING-SPEC 9.2): 8.2 is not met in full.

- One Rule is not active on the toolchain's own source: `gosec::G204` is switched off for all code
  with its reason (see "Switch-offs"); `gqaci_shell_exec` holds the shell-injection hazard it
  guards against.
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
