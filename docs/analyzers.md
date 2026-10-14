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

The tool is chosen by `[analyzers] driver` in `qa/go-qa.toml`.

| `driver`                 | The vet tool                                                                                                                     |
| ------------------------ | -------------------------------------------------------------------------------------------------------------------------------- |
| unset (default)          | the `go-qa` binary itself, started with `GQACI_VETTOOL=1` in its environment; it then registers the bundled analyzers (none yet) |
| `"./internal/qa/driver"` | the project's own package, built with `go build` into a temporary file for the run and deleted afterwards                        |

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

`driver.Main` wraps `unitchecker.Main` and never returns. The driver lists only the analyzers it is
given; the project keeps the bundled ones by passing them too once any exist.

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

## Listing and documentation

`go-qa rules` lists the registered analyzers, read from the tool's `-flags` output (an analyzer is
registered exactly when the tool offers its enabling flag), with the first line of each `Doc`:

```text
analyzer  project_nopanic  forbids panic
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

- No analyzers are bundled yet, so the default tool registers none and the `analyzers` tool passes
  an ordinary project vacuously; the bundled set is a later milestone.
- `go-qa rules` lists only analyzers; the complete listing is a later milestone.
- The docs detector does not know the registered analyzers, so a project page for an analyzer in
  `docs_dir` is reported as an orphan unless a record entry names the analyzer, and a missing page for
  an analyzer is not reported as dangling. `rule-doc` resolves the analyzer's `Doc` regardless.
- A project driver is built with whatever `go.mod` the project has; go-qa does not check that the
  driver's go-qa-ci version matches the running binary.
