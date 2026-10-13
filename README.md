# go-qa-ci

A Defence Before Fix (DBF) Toolchain for Go modules. Work in progress: no conformance to the DBF
Tooling Specification is claimed. No declaration or gap list exists yet; one is added when the Toolchain has something to declare.

## Status

The command skeleton, the configuration loader and the tool catalogue exist (`go-qa --version`,
`go-qa tools`), and `go-qa run` orders and stops the pipeline. The project record loads and
validates (`go-qa record`, the `record` tool; see `docs/record.md`). Identifiers resolve offline
to bundled or project pages (`go-qa rule-doc`, `go-qa docs-check`, the `docs` tool; see
`docs/defences.md`). Every other tool engine is not
built yet, so `go-qa run` reports each of them as failed (not implemented yet); nothing is claimed
to pass. See `docs/PUBLICATION-CHECKLIST.md` before this repository is made public.

## Commands

Global flags: `--root DIR` (default: the directory of the active `go.work`, else of the enclosing
`go.mod`) and `--config FILE` (default `qa/go-qa.toml`, relative to the root). Exit codes: 0
success, 1 a defence or tool failed, 2 usage or configuration error.

| Command                                                               | What                                                                                     |
| --------------------------------------------------------------------- | ---------------------------------------------------------------------------------------- |
| `go-qa run [-t tool]... [-p path]... [--ci] [--no-fix] [--fail-fast]` | the gate: selected tools in phase order, a summary table, and the DBF pointer on failure |
| `go-qa tools`                                                         | the tool catalogue in execution order, with phase and enabled or disabled                |
| `go-qa record check`                                                  | validate the project record; findings are `gqaci.record.invalid`                         |
| `go-qa record list [--json]`                                          | every exception with its justification, enumerated like the defences                     |
| `go-qa record explain <rule or number>`                               | one exception (or every exception for a rule) in full                                    |
| `go-qa rule-doc <id>`                                                 | the page for an identifier, offline; exit 1 when it is dangling                          |
| `go-qa docs-check`                                                    | the docs detector: dangling identifiers, orphan pages, malformed bundled pages           |
| `go-qa --version`                                                     | the version                                                                              |

`run` executes every enabled tool, or only those named with `-t` (an unknown name lists the known
ones). `-p` limits the run to a file or directory under the root. A format or detector failure
stops the run before the first runner; `--fail-fast` stops at the first failure of any kind. `--ci`
(implied by a `CI` variable that is set and not `false` or `0`) means autofix, then fail if the
tree changed; `--no-fix` only checks formatting.

The phases run in order: `format`, then `detectors`, then `runners`. The catalogue is `fmt`;
`record`, `suppression`, `summary`, `docs`, `ast-grep`, `golangci`, `analyzers`, `deadcode`;
`vuln`, `mod`, `test`, `coverage`, `mutants`.

## Configuration

`qa/go-qa.toml` (optional; unknown keys are rejected with exit code 2 and the key named). These
are the defaults, and a test keeps this listing equal to the built-in values:

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

[coverage]
fail_under_statements = 80   # percent of statements covered, 0 to 100

[mutants]
mode = "off"                 # off | diff | full
base = "origin/main"         # diff mode compares against the merge base with this ref
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
inside the root; each `modules` entry must hold a `go.mod`. `[[stage]]`, `[[resolver]]` and the
analyzer driver setting are not accepted yet and are rejected as unknown keys until they are built.

## Public safety

`scripts/public-safety-scan.sh` fails on any term from a denylist kept outside the repository, and
`scripts/secret-scan.sh` runs gitleaks. Enable the hooks with `git config core.hooksPath .githooks`.
The CI workflow requires the `GQACI_DENYLIST` repository secret and fails without it.
