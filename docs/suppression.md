# The suppression detector

The `suppression` tool is a Blocking defence (TOOLING-SPEC 4.3): it fails on every route by which a
finding can be silenced without an entry in the project record. An owner cannot decide what they are
never shown, so the toolchain does not leave the detectors' own escape hatches open.

`go-qa run -t suppression` scans every `.go` file, every `staticcheck.conf` and every
`.golangci.{yml,yaml,toml,json}` under the root (nested modules included). `.git`, `vendor`,
`testdata` and `node_modules` are not walked (the go tool ignores `testdata`), and `scan_exclude`
globs are honoured. `-p` limits the scan to the named paths.

## The findings

| Identifier                     | Meaning                                                                        | Record can license it |
| ------------------------------ | ------------------------------------------------------------------------------ | --------------------- |
| `gqaci.suppression.unrecorded` | a suppression whose rule and file no `covers = "suppressions"` exception names | yes, by that entry    |
| `gqaci.suppression.bare`       | a suppression that names no rule, or gives no reason (the next section)        | no                    |
| `gqaci.record.stale`           | a `covers = "suppressions"` exception that licenses nothing the scan found     | removed by the owner  |

An unparseable file is reported as `gqaci.suppression.unrecorded` with `cannot parse` and the parser's
own `file:line:column` position, whether or not the record names it: what the file silences is unknown,
so it cannot be licensed.

Staleness is judged only on a run over the whole project (no `-p`), because only this tool applies
suppression exceptions and a scoped scan sees only some of them. Exceptions with
`covers = "findings"` are judged by the detectors that apply them, not here.

## The routes, and the rule to record

A route is licensed by an exception whose `rule` is the text below and whose `path` is the file that
carries it. `covers` is left at its default, `suppressions`.

| Route                               | Rule recorded                                                                             |
| ----------------------------------- | ----------------------------------------------------------------------------------------- |
| golangci-lint disabling directive   | the linter, e.g. `errcheck` (one per linter named)                                        |
| staticcheck line or file ignore     | `staticcheck::SA1019` for a staticcheck code, else the check name as written              |
| gosec marker                        | `gosec::G104` (one per code named)                                                        |
| `ast-grep` ignore comment           | the ast-grep rule id as written                                                           |
| generated-code header               | `go::generated-header`                                                                    |
| `staticcheck.conf`, a removed check | `staticcheck.conf::checks::SA1019`                                                        |
| `staticcheck.conf`, a narrowed list | `staticcheck.conf::checks` (a `checks` list without `all` or `inherit`)                   |
| `staticcheck.conf`, a whitelist     | `staticcheck.conf::initialisms`, `::dot_import_whitelist`, `::http_status_code_whitelist` |
| golangci-lint configuration         | `golangci.yml::` plus the route below, whatever the file format                           |

Configuration routes (all under `golangci.yml::`; `X` is the value or item named):

| Setting                                                                                                                                            | Rule                                                                   |
| -------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------- |
| `linters.exclusions.rules` entry                                                                                                                   | `exclusions.rules::<linters>[/<text>]`                                 |
| `linters.exclusions.paths`, `.presets`                                                                                                             | `exclusions.paths::X`, `exclusions.presets::X`                         |
| `linters.exclusions.generated` other than `disable`                                                                                                | `exclusions.generated::X`                                              |
| `formatters.exclusions.paths`, `.generated` other than `disable`                                                                                   | `formatters.exclusions.paths::X`, `formatters.exclusions.generated::X` |
| `linters.disable` item                                                                                                                             | `linters.disable::X`                                                   |
| `linters.default` other than `standard` or `all`                                                                                                   | `linters.default::X`                                                   |
| `severity.rules` entry that is not `error`; `severity.default` other than `error`                                                                  | `severity.rules::<linters>/<severity>`, `severity.default::X`          |
| `issues.max-issues-per-linter`, `issues.max-same-issues` above 0                                                                                   | `issues.<key>::<value>`                                                |
| `issues.new`, `new-from-rev`, `new-from-merge-base`, `new-from-patch`                                                                              | `issues.new`, `issues.new-from-rev::X`, and so on                      |
| `run.tests: false`                                                                                                                                 | `run.tests`                                                            |
| `errcheck.exclude-functions`, `gosec.excludes`, `govet.disable`, `gocritic.disabled-checks`, `gocritic.disabled-tags`, `testifylint.disable` items | `<linter>.<setting>::X`                                                |
| `staticcheck.checks` item starting with `-`                                                                                                        | `staticcheck.checks::<check>`                                          |
| `govet.disable-all`, `testifylint.disable-all`, `revive.ignore-generated-header` set to true                                                       | the setting name                                                       |
| `revive` rule with `disabled: true` or a non-empty `exclude`                                                                                       | `revive.rules::<name>/disabled`, `revive.rules::<name>/exclude`        |

A `<linters>` detail is the linters joined by `+`, or `*` for none. The record cannot bind an entry
to one exclusion among several in a file: the path of the exception is the configuration file, so
the justification must state which exclusion it covers. A TOML configuration has no usable lines
(it is converted before reading), so its findings point at line 0.

The default issue caps of golangci-lint (50 per linter, 3 of the same issue) would hide findings, but
`go-qa run` passes both as 0 on the command line, which overrides the file, so a configuration that
leaves them unset hides nothing from this toolchain and is no route. A cap set above 0 is a route:
it states an intent to hide, and it would hide findings if the file were run by golangci-lint alone.

## Bare suppressions

`gqaci.suppression.bare` fails even when the record names the rule:

- a disabling directive naming no linter, naming `all` (which silences every linter exactly as a
  bare directive does), or naming linters but giving no `// reason`;
- a staticcheck ignore naming no check or giving no reason;
- a gosec or ast-grep marker naming no rule.

## Where the generated-header check lives

In this detector, not in an analyzer. A `// Code generated ... DO NOT EDIT.` header (and the other
phrases golangci-lint's lax generated-file matching accepts: "do not edit", "autogenerated file")
before the package clause makes golangci-lint and staticcheck skip the file, so a hand-written
header is a suppression route. Syntax suffices to see it, so it needs no type information, and it
must be licensed through the record like every other route. Every file with such a header needs an
exception (a `policy = true` glob where a generator emits many); a `//go:generate` directive does
not account for it, because a directive cannot prove that a generator wrote the file. The matching
is deliberately as wide as golangci-lint's, so a doc comment before the package clause that says
"do not edit" is also a finding: reword it.

## Recorded gaps (TOOLING-SPEC 9.2)

go-qa does not claim conformance with 4.3; these routes are not covered:

- `ast-grep-ignore` is read in Go comments only, not in the other languages an ast-grep rule reads.
- The list of golangci-lint per-linter settings that switch a check off is closed (the tables
  above); a setting of a linter not listed there is not seen.
- A configuration passed with `-c` or from outside the root, `//go:build ignore`, and `t.Skip`
  without a condition are not covered.
- The configuration the bundled baseline supplies when a project has none is not scanned; the
  baseline disables the `govet` `fieldalignment` analyzer. This repository records that exception
  against its own `.golangci.yml`.
- Only files under the root are scanned; vendored and `testdata` trees are skipped by design.
- Every `linters.exclusions.rules` entry with neither `linters` nor `text` (a path-only or
  source-only rule) is the same rule, `exclusions.rules::*`, so one record entry licenses any
  number of them in the file; the justification has to say which exclusions it covers.
- A golangci-lint configuration keeps no per-exclusion identity beyond that detail, so the
  one-decision-per-exclusion property holds only for exclusions that name a linter or a text.
