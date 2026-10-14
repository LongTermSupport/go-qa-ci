<!-- BEGIN go-qa-ci generated defences: run `go-qa summary` to refresh; do not edit -->

Active defences. Failures print these ids as listed, except for the code-prefixed golangci linters (gocritic, gosec, govet, revive, staticcheck): a failure prints linter::code (for example staticcheck::SA1019) while the list shows the linter name only. Each entry ends with the command that prints its full documentation. Exceptions only via the project record, with a justification.

- `gqaci.docs.dangling` (builtin, error): every listed identifier resolves to documentation. Fix: Add the page. For a project rule, create `<docs_dir>/<identifier>.md` (`docs/defences` by default) whose first line is `# <identifier>` and which has a `## How to fix correctly` section stating, specifically enough to act on, what to write instead. For a bundled page, correct the first line or add the section in `ruledocs/rules/`. Docs: `go-qa rule-doc gqaci.docs.dangling`
- `gqaci.docs.orphan` (builtin, error): every project documentation page belongs to an identifier the project can see. Fix: Delete the page if its defence is gone. If the defence was renamed, rename the page to the identifier it now prints. If the defence should still run, restore it so that it is listed again. Docs: `go-qa rule-doc gqaci.docs.orphan`
- `gqaci.record.budget` (builtin, error): the project record stays within its budgets. Fix: Remove exceptions by fixing the code they excuse, then prune them from the record. Do not raise a budget to make the gate pass: raising one is a decision for the owner, made in `qa/go-qa.toml` and signed through the ratchet. Docs: `go-qa rule-doc gqaci.record.budget`
- `gqaci.record.expired` (builtin, error): no project-record exception is past its review date. Fix: Re-read the justification against today's code. If the hazard is still accepted, re-decide the entry: set a fresh `decided_on` and a `review_by` inside the horizon of the author's tier. If the code no longer needs it, delete the entry. Better still, fix the code the rule flags and remove the exception. Docs: `go-qa rule-doc gqaci.record.expired`
- `gqaci.record.invalid` (builtin, error): every project-record exception is well formed, attributed and justified. Fix: Edit the entry so that it states a decision. Name the hazard the rule guards against, say why it does not apply here, and give the scope (one file, or the glob and why it is that wide). Fill in `decided_by`, `added_by_model` (`human` or a model id with a configured prefix), `decided_on` and a `review_by` inside the horizon. If you cannot write an honest justification, delete the entry and fix the code the rule flags instead. Docs: `go-qa rule-doc gqaci.record.invalid`
- `gqaci.record.ratchet` (builtin, error): the project record grows only by a signed decision. Fix: If the record shrank, lower `high_water` to the count (outside CI `go-qa run` does it for you; in CI commit the lowered value). To raise the ceiling, a configured decider updates `decided_by`, sets `decided_on` to today and writes a `reason` naming why the raise is needed. If `history_base` does not resolve, set it to a ref that exists and fetch enough history for it. Docs: `go-qa rule-doc gqaci.record.ratchet`
- `gqaci.record.stale` (builtin, error): every project-record exception still covers something. Fix: Delete the exception from the record (`go-qa record prune <number>` removes it and lowers the ratchet ceiling). If the suppression moved, update `path` to its new location and re-read the justification to confirm it still holds. Docs: `go-qa rule-doc gqaci.record.stale`
- `gqaci.rules.unnamed` (builtin, error): every project ast-grep rule has an id. Fix: Give the rule an `id` that is stable and meaningful (not derived from its position), and add a page named after it in the project documentation directory. Docs: `go-qa rule-doc gqaci.rules.unnamed`
- `gqaci.rules.untested` (builtin, error): every project ast-grep rule has valid and invalid test cases. Fix: Add a test file for the rule listing at least one `valid` and one `invalid` snippet, and run `ast-grep test` until it passes. Docs: `go-qa rule-doc gqaci.rules.untested`
- `gqaci.summary.stale` (builtin, error): the generated agent summary is current. Fix: Run `go-qa summary` and commit the regenerated region. Do not edit the region by hand: it is overwritten. Docs: `go-qa rule-doc gqaci.summary.stale`
- `gqaci.suppression.bare` (builtin, error): every suppression names the rule it silences. Fix: Remove the directive and fix the code. If a single named rule genuinely does not apply, name it in the directive and record the exception. Docs: `go-qa rule-doc gqaci.suppression.bare`
- `gqaci.suppression.unrecorded` (builtin, error): every suppression has an exception in the project record. Fix: Fix the code so the finding disappears, and delete the directive. Only where the finding is a true false positive for this project, add an exception to the record that names the rule and the exact path, with a justification stating the hazard and why it does not apply. The finding prints the rule and path to record. An unparseable file is fixed, not recorded. Docs: `go-qa rule-doc gqaci.suppression.unrecorded`
- `bodyclose` (golangci, error): Checks whether HTTP response body is closed successfully. Docs: `golangci-lint help linters`
- `contextcheck` (golangci, error): Check whether the function uses a non-inherited context. Docs: `golangci-lint help linters`
- `depguard` (golangci, error): Go linter that checks if package imports are in a list of acceptable packages. Docs: `golangci-lint help linters`
- `durationcheck` (golangci, error): Check for two durations multiplied together. Docs: `golangci-lint help linters`
- `errcheck` (golangci, error): Errcheck is a program for checking for unchecked errors in Go code. These unchecked errors can be critical bugs in some cases. Docs: `golangci-lint help linters`
- `errchkjson` (golangci, error): Checks types passed to the json encoding functions. Reports unsupported types and reports occurrences where the check for the returned error can be omitted. Docs: `golangci-lint help linters`
- `errname` (golangci, error): Checks that sentinel errors are prefixed with the `Err` and error types are suffixed with the `Error`. Docs: `golangci-lint help linters`
- `errorlint` (golangci, error): Find code that can cause problems with the error wrapping scheme introduced in Go 1.13. Docs: `golangci-lint help linters`
- `exhaustive` (golangci, error): Check exhaustiveness of enum switch statements. Docs: `golangci-lint help linters`
- `forcetypeassert` (golangci, error): Find forced type assertions. Docs: `golangci-lint help linters`
- `gocognit` (golangci, error): Computes and checks the cognitive complexity of functions. Docs: `golangci-lint help linters`
- `gocritic` (golangci, error): Provides diagnostics that check for bugs, performance and style issues. Docs: `golangci-lint help linters`
- `gosec` (golangci, error): Inspects source code for security problems. Docs: `golangci-lint help linters`
- `govet` (golangci, error): Vet examines Go source code and reports suspicious constructs. It is roughly the same as 'go vet' and uses its passes. Docs: `golangci-lint help linters`
- `ineffassign` (golangci, error): Detects when assignments to existing variables are not used. Docs: `golangci-lint help linters`
- `misspell` (golangci, error): Finds commonly misspelled English words. Docs: `golangci-lint help linters`
- `nilerr` (golangci, error): Find the code that returns nil even if it checks that the error is not nil. Docs: `golangci-lint help linters`
- `nilnil` (golangci, error): Checks that there is no simultaneous return of `nil` error and an invalid value. Docs: `golangci-lint help linters`
- `noctx` (golangci, error): Detects function and method with missing usage of context.Context. Docs: `golangci-lint help linters`
- `nolintlint` (golangci, error): Reports ill-formed or insufficient nolint directives. Docs: `golangci-lint help linters`
- `predeclared` (golangci, error): Find code that shadows one of Go's predeclared identifiers. Docs: `golangci-lint help linters`
- `revive` (golangci, error): Fast, configurable, extensible, flexible, and beautiful linter for Go. Drop-in replacement of golint. Docs: `golangci-lint help linters`
- `rowserrcheck` (golangci, error): Checks whether Rows.Err of rows is checked successfully. Docs: `golangci-lint help linters`
- `sqlclosecheck` (golangci, error): Checks that sql.Rows, sql.Stmt, sqlx.NamedStmt, pgx.Query are closed. Docs: `golangci-lint help linters`
- `staticcheck` (golangci, error): It's the set of rules from staticcheck. Docs: `golangci-lint help linters`
- `testifylint` (golangci, error): Checks usage of github.com/stretchr/testify. Docs: `golangci-lint help linters`
- `thelper` (golangci, error): Thelper detects tests helpers which do not start with the t.Helper() method. Docs: `golangci-lint help linters`
- `tparallel` (golangci, error): Tparallel detects inappropriate usage of t.Parallel() method in your Go test codes. Docs: `golangci-lint help linters`
- `unconvert` (golangci, error): Remove unnecessary type conversions. Docs: `golangci-lint help linters`
- `unparam` (golangci, error): Reports unused function parameters. Docs: `golangci-lint help linters`
- `unused` (golangci, error): Checks Go code for unused constants, variables, functions and types. Docs: `golangci-lint help linters`
- `usestdlibvars` (golangci, error): A linter that detect the possibility to use variables/constants from the Go standard library. Docs: `golangci-lint help linters`
- `wrapcheck` (golangci, error): Checks that errors returned from external packages are wrapped. Docs: `golangci-lint help linters`
- `gqaci_sensitive_format` (analyzer, error): gqaci_sensitive_format forbids printing a sensitive value without redaction. Fix: Give the sensitive type a method that prints a fixed placeholder, so every use of it is safe and no call site has to remember: Docs: `go-qa rule-doc gqaci_sensitive_format`

Gate steps, in order (`go-qa tools`): fmt, record, suppression, summary, docs, ast-grep, golangci, analyzers, deadcode, vuln, mod, test, coverage.

Project-record exceptions (prefer fixing the code; an exception names its hazard and scope):

- `golangci.yml::govet.disable::fieldalignment` at `.golangci.yml` (review by 2026-11-13, added by claude-sonnet-5-5)
- `golangci.yml::gosec.excludes::G204` at `.golangci.yml` (review by 2026-11-13, added by claude-sonnet-5-5)
- `golangci.yml::exclusions.rules::gosec/G306` at `.golangci.yml` (review by 2026-11-13, added by claude-sonnet-5-5)

Record budget (`go-qa record list`):

- exceptions: 3 of 10
- per rule: at most 3 (busiest `golangci.yml::exclusions.rules::gosec/g306` with 1)
- policy-level (glob): 0 of 0
- ratchet: high_water 3
- review horizon: 180 days

<!-- END go-qa-ci generated defences -->
