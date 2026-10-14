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

| `driver`                 | The vet tool                                                                                                                  |
| ------------------------ | ----------------------------------------------------------------------------------------------------------------------------- |
| unset (default)          | the `go-qa` binary itself, started with `GQACI_VETTOOL=1` in its environment; it then registers the bundled analyzers (below) |
| `"./internal/qa/driver"` | the project's own package, built with `go build` into a temporary file for the run and deleted afterwards                     |

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

There are 9 bundled analyzers. Each is registered by `analyzers/pack` (the one list the vet tool, the
driver and the tests read), is type-based (it resolves `go/types` objects, so an import alias, a
method value or a function stored in a variable does not escape it, and a reference counts as well as
a call), has a page under `ruledocs/rules/`, and is proven with `analysistest` fixtures that fail
without the rule.

| Identifier                 | Blocks                                                                                                                   | Page                                         |
| -------------------------- | ------------------------------------------------------------------------------------------------------------------------ | -------------------------------------------- |
| `gqaci_defer_in_loop`      | a `defer` inside a loop body, which runs at function exit and piles up one call per iteration                            | `ruledocs/rules/gqaci_defer_in_loop.md`      |
| `gqaci_exec_context`       | `exec.Command` where a context (or a `*testing.T`) is in scope, so cancellation cannot stop the child                    | `ruledocs/rules/gqaci_exec_context.md`       |
| `gqaci_exit_outside_main`  | `os.Exit`, `syscall.Exit` and `log.Fatal*` outside `main` of package `main` (and `TestMain`)                              | `ruledocs/rules/gqaci_exit_outside_main.md`  |
| `gqaci_init_side_effects`  | an `init` function that does input or output, runs a program, starts a goroutine or changes the process                  | `ruledocs/rules/gqaci_init_side_effects.md`  |
| `gqaci_map_range_order`    | output, or a returned slice, whose order is decided by ranging over a map                                                | `ruledocs/rules/gqaci_map_range_order.md`    |
| `gqaci_sensitive_format`   | a value holding a sensitive-named type printed by fmt, log or slog without a redacting method                            | `ruledocs/rules/gqaci_sensitive_format.md`   |
| `gqaci_shell_exec`         | a shell started with `-c` by constant argument analysis, the evasion that hides a command from argument-based checks     | `ruledocs/rules/gqaci_shell_exec.md`         |
| `gqaci_test_clock`         | `time.Now`, `time.Since` and `time.Until` in a `_test.go` file                                                           | `ruledocs/rules/gqaci_test_clock.md`         |
| `gqaci_unconditional_skip` | a `t.Skip`, `t.Skipf` or `t.SkipNow` at the top level of a test body, so the test never runs                             | `ruledocs/rules/gqaci_unconditional_skip.md` |

Every analyzer is Blocking (none has an advisory mode) and its only exception is a project-record
entry with `covers = "findings"`, the analyzer name as `rule` and a path glob; there is no comment
route, so an allow-list (the test files that may read the clock, the packages whose `init` may start a
goroutine) is a set of record entries, each with a justification and a review date. Only
`gqaci_sensitive_format` is configurable, through its `-patterns` and `-redacting` flags (defaults in
`sensitiveformat.DefaultConfig`). `tests/pack` keeps this section, the README and `internal/builtin`
in step with the registry.

Rules left out on purpose are listed in the recorded gaps below.

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

- Only `gqaci_sensitive_format` is configurable, through its flags or `sensitiveformat.New` in a
  project driver; `qa/go-qa.toml` has no key for it yet.
- Not shipped: a goroutine-leak rule (a `go` statement with no context, wait group or error group
  in scope) is not decidable from one function without many false findings, and a rule for
  `//go:build ignore` is a suppression-scanner matter (`docs/suppression.md`), not an analyzer. The
  syntactic candidates were checked against the pack's rule (syntax must provably suffice): every
  bundled rule needs types (the callee, the context in scope, the constant argument) or a flow
  question (is the slice sorted afterwards), so the `gqaci.ast.*` pack stays empty.
- `gqaci_map_range_order` and `gqaci_init_side_effects` read one function at a time: a slice passed
  on through other variables or functions, and an effect hidden behind a call to a function of the
  same package, are missed, in exchange for few false findings.
- The docs detector does not know the registered analyzers, so a project page for an analyzer in
  `docs_dir` is reported as an orphan unless a record entry names the analyzer, and a missing page for
  an analyzer is not reported as dangling. `rule-doc` resolves the analyzer's `Doc` regardless.
- A findings exception whose rule is not a registered analyzer is never reported stale by an
  analyzers run, so a mistyped analyzer name, or an analyzer removed from the driver, leaves a dead
  entry; golangci-lint findings exceptions are not judged for staleness either (docs/record.md).
- `-p` filters findings only after the whole module has been analysed (`go vet ./...` always
  runs), so a single-file run costs as much as a full run; DETECTOR-SPEC 5.2 is met in effect, not
  in cost.
- The `[analyzers] driver` wiring (`internal/cli/analyzers.go`) is proven in pieces: the CLI lists
  the test binary's analyzer, and `internal/vet` builds a project driver and runs it. No CLI test
  sets `driver` and then runs `go-qa rules` or `run -t analyzers` end to end; that was checked by
  hand with the real binary.
- A project driver is built with whatever `go.mod` the project has; go-qa does not check that the
  driver's go-qa-ci version matches the running binary.
