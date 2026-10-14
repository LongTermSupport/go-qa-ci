# go-qa-ci

One QA entry point for any Go module, built for the Defence Before Fix (DBF) method
(https://defence-before-fix.github.io/): bespoke rules on Go's type information and syntax,
blocking enforcement through a single command, offline rule documentation, a project record for
every exception, and an enumerable list of active defences. It is a generic toolchain: it knows
nothing of the projects that use it, and a project adapts it through `qa/go-qa.toml`, a project
record, its own analyzers and ast-grep rules, and its own stages.

Status: early. go-qa-ci **does not claim conformance** to the DBF Tooling Specification. The two
levels of TOOLING-SPEC 9.1 (the project, and the Toolchain it ships) are graded separately in
[docs/CONFORMANCE.md](docs/CONFORMANCE.md), clause by clause against method 1.1.0, detector 1.0.0
and toolchain 0.2.0, and every known gap is recorded in the machine-readable declaration
[defence-before-fix.toml](defence-before-fix.toml) (TOOLING-SPEC 9.2); a test keeps the two in
step. The repository is private until the checklist in
[docs/PUBLICATION-CHECKLIST.md](docs/PUBLICATION-CHECKLIST.md) has been run. Licence: MIT
([LICENSE](LICENSE)).

## What it runs

`go-qa run` executes three phases in a fixed order. Every detector runs so one invocation reports
all static findings; any format or detector failure stops the run before the first runner. A tool
that is switched off does not run (`go-qa tools` shows it as disabled) and a step with no engine
fails as not implemented; neither counts as a pass. Within a phase the built-in tools run first, then the project's own stages (see
"Project stages").

| Phase     | Tool (`-t` name) | What                                                                                                                                                                                |
| --------- | ---------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| format    | `fmt`            | the formatters of the golangci-lint configuration (`gofumpt`, `goimports`); fixes locally, `--no-fix` checks                                                                        |
| detectors | `record`         | project record validation and anti-bloat policy (`gqaci.record.invalid`, `.budget`, `.ratchet`, `.expired`, `.stale`)                                                               |
|           | `suppression`    | every route that silences a defence without a record entry (`gqaci.suppression.unrecorded`)                                                                                         |
|           | `summary`        | agent summary freshness (`gqaci.summary.stale`)                                                                                                                                     |
|           | `docs`           | every listed identifier resolves to documentation (`gqaci.docs.dangling`), every page documents a listed identifier (`gqaci.docs.orphan`)                                           |
|           | `ast-grep`       | syntactic rules: the project's `sgconfig.yml` rules, then the bundled `gqaci.ast.*` pack; every project rule has an id (`gqaci.rules.unnamed`) and a proof (`gqaci.rules.untested`) |
|           | `golangci`       | golangci-lint over a strict bundled baseline; every finding prints its linter (and check code)                                                                                      |
|           | `analyzers`      | `go/analysis` analyzers through `go vet -vettool`: the 9 bundled analyzers (docs/analyzers.md) and the project's own driver; every finding prints its analyzer name                        |
|           | `deadcode`       | functions unreachable from every `main` function and test (modules with a `main` package)                                                                                           |
| runners   | `vuln`           | govulncheck, symbol-level                                                                                                                                                           |
|           | `mod`            | `go mod tidy -diff`, then `go mod verify`                                                                                                                                           |
|           | `test`           | `go test -race -shuffle=on -count=1 ./...`                                                                                                                                          |
|           | `coverage`       | statement coverage with a threshold                                                                                                                                                 |
|           | `mutants`        | gremlins (`off`, `diff` against a base, or `full`)                                                                                                                                  |

Options: `-t <tool or stage>` (repeatable), `-p <path>` (repeatable; a file or a directory under
the root; one file is enough), `--ci` (also implied by a `CI` variable that is set and not `false`
or `0`: autofix, then fail if the working tree changed), `--no-fix` (only check formatting),
`--fail-fast` (stop at the first failure of any kind), `--changed [--base <ref>]` (diff mode, see
below). An unknown `-t` name lists the known ones.
Exit codes: 0 success, 1 a defence or tool failed, 2 usage or configuration error. A required
program that is not on `PATH` is an error, never a skipped check.

Global flags: `--root DIR` (default: the directory of the active `go.work`, else of the enclosing
`go.mod`) and `--config FILE` (default `qa/go-qa.toml`, relative to the root).

### Diff mode

`go-qa run --changed [--base <ref>]` is for fast local iteration. The full run stays the gate:
run it before every push, and in CI. `--changed` cannot be combined with `-p`, and `--base` needs
`--changed`.

The changed set is the files changed against `HEAD` plus the untracked files that are not
ignored; with `--base <ref>` it is the files changed since the merge base of `<ref>` and `HEAD`,
plus the working tree (so a committed change on a branch is covered).

| Defences                                                                  | In diff mode                                                                                                                             |
| ------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------- |
| per-package engines: `golangci`, `analyzers`, `test`, `coverage`          | run on the packages containing a changed `.go` file and on the packages that import them, directly or not, within the module (`go list`) |
| `deadcode`, `mutants` (whole-program)                                     | run as configured when any package is affected                                                                                           |
| file-scoped: `fmt`, `suppression`, `ast-grep`, stages that take `{files}` | run on the changed files only (`fmt` on the changed `.go` files)                                                                         |
| repo-wide: `record`, `docs`, `summary`                                    | always run, over the whole project                                                                                                       |
| `coverage` (narrowed)                                                     | the module-wide `fail_under_statements` threshold is applied to the affected packages only, so the result can differ from the full run's |
| `vuln`, `mod` (whole-module)                                              | run as configured when any package is affected (a changed Go file can start calling a vulnerable function or import an unlisted module)  |

A changed file under a `testdata` directory runs the tests of the package that owns the directory
(not its importers); a changed file a package embeds (`go:embed`) runs that package and its
importers. Any other non-Go file selects no package.

A change to `go.mod`, `go.sum`, `go.work`, the tool lock, a golangci configuration, the project
record, the configuration file or a rule source widens the run to everything, and the summary says
why. A rule source is whatever the configuration names: `sgconfig.yml`, its rule and test
directories, `docs_dir`, and the analyzer module (`analyzers.module`, else the directory of
`analyzers.driver`). Nothing is widened by its directory name alone. So does a changed `.go` file whose package no longer exists. With no changed files the
run says so, skips what is scoped, runs the repo-wide defences and exits 0 if they pass. Because
the scope is narrower, the checks that need a whole-project view (a record exception that
licensed nothing is stale) do not speak in a narrowed run. The run summary opens with the mode
and the scope:

```text
go-qa summary:
mode: changed (files changed against HEAD, plus untracked files that are not ignored)
scope: 1 changed file; packages: ./leaf, ./mid
```

Diff mode makes no claim of conformance on its own: it narrows the work, and a green narrowed run
says nothing about the packages it did not run.

### Further modules

A repository may hold more than one Go module that is not part of a `go.work`: list each under
`modules` in `qa/go-qa.toml` (a relative path holding a `go.mod`). The tools that work per module
(`golangci`, `analyzers`, `vuln`, `mod`, `test`, `coverage`, `mutants`) then run once for each.

Other commands, none of which run a defence:

| Command                                 | What                                                                                                                                                                        |
| --------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `go-qa tools`                           | the tool catalogue in execution order, with phase and enabled or disabled                                                                                                   |
| `go-qa rules [--json]`                  | every active defence, derived from the configuration, with its identifier, one-line statement and documentation route; also the record and its budget (docs/enumeration.md) |
| `go-qa rule-doc <id>`                   | the page for an identifier, offline; exit 1 when it is dangling                                                                                                             |
| `go-qa docs-check`                      | the docs detector: dangling identifiers, orphan pages, malformed bundled pages                                                                                              |
| `go-qa record check`                    | validate the project record and its policy; findings are `gqaci.record.invalid`, `.budget`, `.expired` and `.ratchet`                                                       |
| `go-qa record prune <number>...`        | remove the exceptions numbered as `record list` shows them, and lower the ratchet to what remains                                                                           |
| `go-qa record list [--json]`            | every exception with its justification, enumerated like the defences                                                                                                        |
| `go-qa record explain <rule or number>` | one exception (or every exception for a rule) in full                                                                                                                       |
| `go-qa summary [--check]`               | write the generated defence region into `[summary] file`; `--check` fails as `gqaci.summary.stale` when it is out of date                                                   |
| `go-qa --version`                       | the version                                                                                                                                                                 |

## Writing rules

Write each rule against the richest layer that can express it, and give it an identifier, a page
and a red proof.

1. A `golang.org/x/tools/go/analysis` analyzer, for anything that needs types, scope or flow. The
   project keeps its analyzers in a small `main` package (a driver) and names it in
   `[analyzers] driver` (and, when it lives in a nested module so that its dependencies stay out of
   the project's own `go.mod`, `[analyzers] module`); go-qa runs it through `go vet -vettool`, prints each finding under the
   analyzer name and applies the project record. Prove each analyzer with `analysistest` against a
   `testdata` fixture. See [docs/analyzers.md](docs/analyzers.md).
2. An ast-grep rule, for a cheap single-file syntactic check, in the project's `sgconfig.yml` rule
   directories. Every rule needs an `id` and a test case with `valid` and `invalid` code. See
   [docs/astgrep.md](docs/astgrep.md), including how to write a Go call pattern.
3. Configuration of golangci-lint, for a check an existing linter already makes. Do not switch a
   linter off in a comment: a suppression is a record entry (see "Project record").

Each identifier a defence prints resolves offline: `go-qa rule-doc <id>` prints the page for it,
and `go-qa docs-check` fails when a built-in, stage, stage rule or project ast-grep rule has no page,
or a page documents none of those, an analyzer of the project's own vet driver or a project record
rule. Golangci-lint linters and the bundled `gqaci.ast.*` rules are not enumerated by it (a recorded gap, 4.2). Project pages
live in `docs_dir` (default `docs/defences`, one `<identifier>.md` each) and must state the correct
construction, not only the prohibition. See [docs/defences.md](docs/defences.md).

## Project stages

A command no built-in tool covers (another language's linter, a check that a checked-in output is
fresh) is declared as a `[[stage]]` in `qa/go-qa.toml` and runs in the phase it names; it is listed
by `go-qa tools` and `go-qa rules`, selected with `-t <name>`, documented under its name, and
subject to the project record like any other defence. `[[resolver]]` maps a glob of identifiers to
documentation text. Both are empty by default; the fields, the `{files}` expansion and the
`status` and `findings` outputs are in [docs/stages.md](docs/stages.md).

## Project record

The project record, `qa/record.toml`, is the only place an exception to a defence is decided. go-qa
loads it itself. Every exception carries a written justification that names the hazard accepted and
the scope, and a generic one ("needed for now", "legacy", "TODO") is rejected by a documented
check; the check cannot verify the sentence is true, which is the Owner's judgement.
`go-qa record list` enumerates the record like the defences. An exception licenses either a
suppression written in the source or configuration (`covers = "suppressions"`) or the findings of
a detector at a path (`covers = "findings"`). The `suppression` tool fails the routes that
silence a defence without such an entry; the routes it does not see are recorded as gaps in
[docs/suppression.md](docs/suppression.md). The format, the exact justification rules and the
`gqaci.record.*` findings are in [docs/record.md](docs/record.md) and
[docs/suppression.md](docs/suppression.md).

### Anti-bloat controls

A record is kept small by policy: `max_total`, `max_per_rule`, `max_policy`, a review deadline
(`max_review_days`, tightened by the trust tier of whoever wrote the entry), a ratchet that fails
when the number of entries rises without a decider's signature, and `go-qa record prune`. The
defaults are in "Configuration" and the mechanics in [docs/record.md](docs/record.md).

## Configuration

`qa/go-qa.toml` (optional; unknown keys, and keys spelt in another case, are rejected with exit
code 2 and the key named; record caps and review horizons cannot be negative; `--config` may be
a path inside the root, relative or absolute). These are the defaults, and a test keeps this listing equal to the built-in values:

```toml
record = "qa/record.toml"    # shorthand for [record] path, with the defaults below
docs_dir = "docs/defences"
scan_exclude = []            # globs excluded from the source scans
modules = []                 # further Go modules (own go.mod, not in a go.work)

[summary]
# file = "AGENTS.md"         # agent-loaded file that receives the generated region; unset = off
fix_headings = ["How to fix correctly"]

[tools]                      # explicit `-t` requests override these switches
fmt = true
record = true
suppression = true
summary = true
docs = true
ast_grep = true
golangci = true
analyzers = true
deadcode = true
vuln = true
mod = true
test = true
coverage = true
mutants = true

[ast_grep]
bundled = true               # run the bundled gqaci.ast.* pack beside the project's own rules

[analyzers]
# driver = "./internal/qa/vetdriver"  # your analyzer driver package; unset = go-qa is the vet tool
# module = "qa"                       # the nested module (own go.mod) holding the driver; unset = the root module

[coverage]
fail_under_statements = 80   # percent of statements covered, 0 to 100

[mutants]
mode = "off"                 # off | diff | full
base = "origin/main"         # the ref diff mode hands to gremlins --diff
threshold_efficacy = 100     # percent of mutants the tests must kill, 0 to 100
```

The record policy can be a `[record]` table instead of the `record = "..."` shorthand:

```toml
[record]
path = "qa/record.toml"
max_total = 10               # entries in the record
max_per_rule = 3             # entries for one rule id
max_policy = 0               # broad entries marked policy = true; at most max_per_rule
max_review_days = 180        # longest decided_on to review_by gap for any tier
deciders = []                # people who sign the ratchet, human entries and co-signs
history_base = "origin/main" # merge base for raise detection, besides HEAD

[record.trust]
models = {}                  # model id prefix -> high | medium | low

[record.trust.human]         # each tier: max_review_days and max_count; unset = record-wide cap
[record.trust.high]
max_review_days = 90
[record.trust.medium]
max_review_days = 30
max_count = 3
[record.trust.low]
max_review_days = 14
max_count = 1

# [[record.trust.cosign]]    # rule categories that need a configured decider below min_tier
# rules = ["errcheck::*"]
# min_tier = "high"
```

Paths in the file (`record`, `docs_dir`, `summary.file`, `modules`) must be relative and stay
inside the root; each `modules` entry must hold a `go.mod`; `analyzers.driver` is a package of the
module written like `./internal/qa/driver`, and `analyzers.module` (a relative directory holding a
`go.mod`) says which module that is when the rules live in a nested one. `[[stage]]` and `[[resolver]]` are described in
[docs/stages.md](docs/stages.md).

## Installing

go-qa-ci pins every external tool in [tools.lock](tools.lock): golangci-lint, ast-grep, gitleaks,
actionlint, deadcode, govulncheck and gremlins. Pin an exact tag of this repository (none is published yet). Install the
binary, then the pinned tools from a checkout of the same tag:

```bash
go install github.com/LongTermSupport/go-qa-ci/cmd/go-qa@<tag>
git clone --depth 1 --branch <tag> https://github.com/LongTermSupport/go-qa-ci /tmp/go-qa-ci
/tmp/go-qa-ci/scripts/install-tools.sh            # versions from tools.lock
/tmp/go-qa-ci/scripts/install-tools.sh --check    # report missing or mismatched tools, exit 1 if any
/tmp/go-qa-ci/scripts/install-tools.sh ast-grep   # only the named tools
```

Tools land in `$GOBIN`, else `$(go env GOPATH)/bin`, which must be on `PATH`. The Go tools compile
from source with `go install` (verified by the Go checksum database); every release archive
(golangci-lint, ast-grep, gitleaks, actionlint) is verified against the sha256 pinned in
`tools.lock`, never against a checksum file published beside it, and a release tool with no pinned
sha256 is refused. Release archives are linux/amd64 only. The Go toolchain itself is the one `go.mod`
requires. This repository's CI installs its tools the same way and then runs `go-qa run --ci`
over itself (job `self-gate`).

## Public-safety guard

This repository must stay safe to publish. Two scans guard it:

- `scripts/public-safety-scan.sh` checks the tree, staged changes, commit messages, the whole history
  (diffs, messages, identities, tag messages, ref names) or arbitrary text against a denylist
  supplied from outside the repository (`GQACI_DENYLIST=/path` or `git config gqaci.denylist /path`).
  The format is in [docs/denylist.example](docs/denylist.example). Findings print a line number,
  never the term.
- `scripts/secret-scan.sh` runs gitleaks over the history, staged changes or tree, with no
  suppression route.

Enable the hooks with `git config core.hooksPath .githooks`. The CI workflow
(`.github/workflows/public-safety.yml`) runs both scans and requires the `GQACI_DENYLIST`
repository secret; it fails without it. The pre-publication audit is
[docs/PUBLICATION-CHECKLIST.md](docs/PUBLICATION-CHECKLIST.md).
