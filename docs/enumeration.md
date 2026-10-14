# Enumeration and agent context: `rules` and `summary`

What is active is asked of go-qa, never read from a hand-kept list (TOOLING-SPEC 5.1, 5.2, 5.3,
6.3, 7.1, 7.2). Both commands derive their answer from the active configuration on every call, so
adding a rule to the configuration (a `[[stage]]`, a rule file in an ast-grep project, a linter in
the golangci-lint configuration, an analyzer in the project's driver) puts it in the listing with
no other edit.

## `go-qa rules [--json]`

Lists the defences the configuration has active, in this order of engines:

| Engine     | Where the identifiers come from                                                                          |
| ---------- | -------------------------------------------------------------------------------------------------------- |
| `builtin`  | the `gqaci.*` identifiers (`internal/builtin`) of every enabled tool that raises them                    |
| `golangci` | `golangci-lint linters --json` under the configuration in force (the project's own, else the baseline)   |
| `analyzer` | the analyzers the vet tool registers (go-qa itself, or `[analyzers] driver`)                             |
| `ast-grep` | the bundled `gqaci.ast.*` pack (when `[ast_grep] bundled`) and the rules of the project's `sgconfig.yml` |
| `stage`    | each `[[stage]]` under its name, and the rules its `rules` command prints                                |

A defence of a tool that is switched off in `[tools]` is not listed. Each entry carries `id`,
`engine`, `origin` (`bundled`, or `project` for what the project wrote or configured), `phase`,
`level` (`error`: every defence blocks; an ast-grep rule reports the severity it declares),
`description` and, where a page or the stage declares one, `fix`: the first paragraph of the
section of the identifier's page named in `[summary] fix_headings`.

The JSON document has five keys:

```json
{
  "defences": [{ "id": "errcheck", "engine": "golangci", "origin": "bundled", "phase": "detectors",
                 "level": "error", "description": "Finds unchecked errors." }],
  "stages":   [{ "name": "fmt", "phase": "format", "project": false, "enabled": true }],
  "runners":  [{ "name": "test", "phase": "runners", "project": false, "enabled": true }],
  "record":   [{ "number": 1, "rule": "errcheck", "path": "p.go", "covers": "suppressions",
                 "justification": "..." }],
  "budget":   { "total": 1, "max_total": 10, "busiest_rule": "errcheck", "busiest_count": 1,
                 "max_per_rule": 3, "policy": 0, "max_policy": 0, "high_water": -1,
                 "max_review_days": 180 }
}
```

`stages` are the format and detector steps and `runners` the runner steps, in execution order,
project stages after the built-in tools of their phase. `record` holds the project record's
exceptions (the same entries as `go-qa record list --json`), and `budget` the record against its
caps; `high_water` is -1 when the record has no ratchet.

A tool that cannot report its rules fails the command (exit 1): golangci-lint missing, an analyzer
driver that does not build, a stage whose `rules` command fails. A partial listing is never printed
as if it were complete.

## `go-qa summary [--check]`

Writes the listing into a delimited region of the file named by `[summary] file`, a file agents
already load (for example `AGENTS.md`). The region lists every active defence with its engine,
level, description, fix and the command that prints its page, then the gate's enabled steps, the
record's exceptions and the record budget. Text outside the markers is never touched; the file is
created when it does not exist, and the region is appended when there is none. Markers that do not
form exactly one pair are an error, so a hand edit can never be overwritten by guesswork.

`go-qa summary --check` (and the `summary` tool of `go-qa run`) writes nothing and fails as
`gqaci.summary.stale` when the file is missing, has no region, or the region differs from what
`go-qa summary` would write now. The remedy is to run `go-qa summary` and commit the result; the
finding is not suppressible. With no `[summary] file`, `go-qa summary` is a usage error (exit 2) and
the `summary` tool passes after saying on stderr that it is off.

The region depends on the installed golangci-lint (its linter descriptions), so CI and developers
use the one pinned version; a different version can render a stale region.

## Recorded gaps (TOOLING-SPEC 9.2)

- The `docs` detector does not take its list of identifiers from this listing; it enumerates
  built-ins, stages, stage rules and the record's rules itself. A page for an ast-grep rule, a
  golangci-lint linter or an analyzer in `docs_dir` is therefore reported as an orphan, and a
  missing page for one of them is not reported as dangling (`docs/defences.md`).
- The listing does not include advisories of `vuln` or the `deadcode` findings: they have no stable
  rule identifiers of their own (`docs/runners.md`).
- `fix` is quoted only from pages the resolver knows (project, stage, bundled and `[[resolver]]`
  pages); a golangci-lint linter has none yet (`docs/golangci.md`), so it lists with no fix.
- There is no per-defence `summarised` switch: the summary lists every active defence.
- 5.1 route partial: every defence in `rules` and the summary carries a `doc` route. A golangci-lint
  linter's route is `golangci-lint help linters`, which prints every linter rather than the one
  named, and prints nothing of a wrapped tool's individual codes; `go-qa rule-doc` is the route only
  where it resolves (bundled and project pages, stages, analyzers). A defence nothing documents has
  an empty `doc` and the summary says so; the `docs` detector does not yet report it as dangling.
