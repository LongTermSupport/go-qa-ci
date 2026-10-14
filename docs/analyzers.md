# Bespoke type-aware rules: the `analyzers` tool

Type-aware bespoke rules are `golang.org/x/tools/go/analysis` analyzers run through `go vet`'s
`-vettool` protocol (DETECTOR-SPEC 4.1 and 4.2, through the go vet register entry). go-qa owns the
run, so the three things plain `go vet` lacks are supplied here:

- every finding is printed under the analyzer name, `error[<analyzer>]` (plain `go vet` prints no
  name);
- `go vet -json` exits 0 whatever the analyzers find, so go-qa reads the findings and decides
  pass or fail itself;
- go vet has no inline suppression route, so the only exception is a project-record entry with
  `covers = "findings"`, applied by go-qa (TOOLING-SPEC 7).

```text
error[project_nopanic]: panic is forbidden
  --> internal/x/x.go:12
```

## The vet tool

The tool is chosen by `[analyzers] driver` (and optionally `module`) in `qa/go-qa.toml`.

| `driver`                 | The vet tool                                                                                                                  |
| ------------------------ | ----------------------------------------------------------------------------------------------------------------------------- |
| unset (default)          | the `go-qa` binary itself, started with `GQACI_VETTOOL=1` in its environment; it then registers the bundled analyzers (below) |
| `"./internal/qa/driver"` | the project's own package, built with `go build` into a temporary file for the run and deleted afterwards                     |

`driver` is a package pattern relative to the module that holds it. That is the root module unless
`module` names another: `module = "qa"` builds the driver with `go build -C qa`, in the Go module
whose `go.mod` is `qa/go.mod`. Use it when the rules live in a nested module, so that
`golang.org/x/tools` and go-qa-ci stay out of the `go.mod` of the project's own binaries, which then
stays free of dependencies:

```toml
[analyzers]
module = "qa"            # a directory with its own go.mod, relative to the root
driver = "./driver"      # the main package, relative to that module
```

`module` must be a relative path inside the root holding a `go.mod`, and needs `driver`. The nested
module is not part of the root module, so `go vet ./...` over the root does not enter it; list it
under `modules` as well to have the tools that work per module (`golangci`, `test`, ...) run over
it too. In `--changed` runs, a change under the module widens the run to the whole project (with no
`module`, a change under the driver package's directory does), because a changed rule must be
re-applied everywhere.

A project driver is a few lines in a `main` package of the project's module, calling the public
`driver` package with its analyzers:

```go
package main

import (
	"github.com/LongTermSupport/go-qa-ci/driver"

	"example.com/project/internal/qa/rules"
)

func main() { driver.Main(rules.NoPanic, rules.NoGlobals) }
```

`driver.Main` wraps `unitchecker.Main` and never returns. It always registers the bundled analyzers
beside the ones it is given, so a project driver cannot switch a Blocking defence off by leaving it
out; to configure the bundled analyzer, pass `sensitiveformat.New(sensitiveformat.Config{...})`,
which replaces the bundled one of the same name. `New` panics on a config with no pattern (an empty
list marks no type sensitive, the case the `-patterns` flag refuses), and `driver.Main` panics on
any other analyzer that takes the bundled name, so neither route leaves the defence off.

Name analyzers with identifier characters only (`project_nopanic`), because the analyzer name is the
identifier printed with every finding and the key of the record entry.

## What a run does

For the root module and each further module in `modules`, `go vet -vettool=<tool> -json ./...`
runs in the module directory. A package that does not compile, a missing `go` binary, a driver that
does not build and an analyzer that reports an error instead of findings are failures of the step:
nothing analysed is never a pass. Findings are sorted by path and line; a finding both a package and
its test variant report is printed once. `-p` filters the findings after the whole module has been
analysed.

A findings exception matches on the analyzer name (`rule`) and a path glob. On a run over the whole
project, a `covers = "findings"` entry whose rule is a registered analyzer and that licensed nothing
fails as `gqaci.record.stale`; entries for other detectors' rules are left to those detectors, and a
run limited with `-p` cannot judge staleness.

## Bundled analyzers

| Identifier               | Blocks                                                                                        | Page                                       |
| ------------------------ | --------------------------------------------------------------------------------------------- | ------------------------------------------ |
| `gqaci_sensitive_format` | a value holding a sensitive-named type printed by fmt, log or slog without a redacting method | `ruledocs/rules/gqaci_sensitive_format.md` |

The analyzer is Blocking (it has no advisory mode) and its only exception is a project-record entry
with `covers = "findings"`. Its configuration is the `-patterns` and `-redacting` flags (defaults in
`sensitiveformat.DefaultConfig`).

## Listing and documentation

`go-qa rules` lists the registered analyzers (with every other engine, `docs/enumeration.md`), read from the tool's `-flags` output (an analyzer is
registered exactly when the tool offers its enabling flag), with the first line of each `Doc`:

```text
  analyzer  project    error  project_nopanic  forbids panic
```

`go-qa rule-doc <analyzer>` prints the analyzer's `Doc`, obtained from the tool's `help <name>`.
Resolution order for an identifier is a project page, a bundled page, then analyzer help, so a
project page overrides the `Doc`. Write the `Doc` as a page: first line the summary, then what is
flagged, why, and how to fix it correctly.

## The single-rule harness

Each analyzer is proven alone with `analysistest` (DETECTOR-SPEC 4.2): a fixture package under
`testdata/src/<pkg>` marks every expected finding with a `// want` comment and leaves the clean
constructs unmarked, so a rule that fires too much or too little fails its own test.

```go
func TestNoPanic(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), rules.NoPanic, "a")
}
```

`internal/vettest` in this repository is the working example: a trivial analyzer, `gqaci_test`,
tested this way, and the `Serve` hook that lets a test binary act as the vet tool.

## Recorded gaps (TOOLING-SPEC 9.2)

These are gaps, not claims:

- One analyzer is bundled so far (`gqaci_sensitive_format`); the rest of the bundled set is a later
  milestone. The bundled analyzer is configured through its flags or `sensitiveformat.New` in a
  project driver; `qa/go-qa.toml` has no key for it yet.
- A findings exception whose rule is not a registered analyzer is never reported stale by an
  analyzers run, so a mistyped analyzer name, or an analyzer removed from the driver, leaves a dead
  entry; golangci-lint findings exceptions are not judged for staleness either (docs/record.md).
- `-p` filters findings only after the whole module has been analysed (`go vet ./...` always
  runs), so a single-file run costs as much as a full run; DETECTOR-SPEC 5.2 is met in effect, not
  in cost.
- Rules a driver imports from a package outside the driver's own directory (and outside `module`)
  do not widen a `--changed` run when they change; keep the rules in the nested module, or beside
  the driver.
- A project driver is built with whatever `go.mod` the project has; go-qa does not check that the
  driver's go-qa-ci version matches the running binary.
