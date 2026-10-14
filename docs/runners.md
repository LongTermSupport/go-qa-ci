# Runners and the deadcode detector

The runners execute code or audit dependencies, after every format and detector step has passed
(TOOLING-SPEC 4.5). They stop at the first failing runner, so a failing `test` keeps `coverage`
and `mutants` from running. Each runs once per Go module the configuration covers (the root module
and `modules`). A required program that is not on `PATH` is an error, never a skipped check.

| Tool                  | Command                                                                               | Fails when                                                                         |
| --------------------- | ------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------- |
| `vuln`                | `govulncheck -format json ./...`                                                      | the code calls a vulnerable symbol the record does not license                     |
| `mod`                 | `go mod tidy -diff`, then `go mod verify`                                             | `go.mod` or `go.sum` is not tidy, or a dependency does not match its recorded hash |
| `test`                | `go test -race -shuffle=on -count=1 ./...`                                            | a test fails or the code does not build                                            |
| `coverage`            | `go test -covermode=atomic -coverpkg=./... -coverprofile`, then `go tool cover -func` | statement coverage is below `coverage.fail_under_statements`                       |
| `mutants`             | `gremlins unleash --config <generated>` (diff mode adds `--diff <base>`)                  | the tests kill fewer than `mutants.threshold_efficacy` percent of the mutants      |
| `deadcode` (detector) | `deadcode -test -json ./...`                                                          | a function is unreachable from every `main` function and test                      |

## Identifiers

- `vuln` prints each finding under the advisory identifier (`GO-YYYY-NNNN`). Scanning is at symbol
  level: a vulnerability in a dependency, or in an imported but never called package, is not a
  finding, because reachability decides. The location is the call site in the module, and an
  advisory reached from two files is two findings, so a record exception (`covers = "findings"`)
  scoped to one file licenses the advisory in that file only; on a run over the whole
  project an exception for an advisory that is no longer reported fails as `gqaci.record.stale`.
- `deadcode` prints findings under `deadcode` and the record licenses them by that name and path.
  It applies only to a module that has a `main` package; for a library it is skipped, saying so,
  and the `unused` linter in the golangci baseline is the defence there.
- `mod`, `test`, `coverage` and `mutants` print `error[mod]`, `error[test]`, `error[coverage]` and
  `error[mutants]` with the module and the tool's own output. They are verdicts on a whole module,
  not findings in a file, so the project record has no exception for them.

## Mutation testing

`mutants.mode` is `off` (the default: the step is skipped, saying so), `diff` or `full`. The
`gremlins unleash` flags used are `--diff <ref>` (verified against gremlins v0.6.0, whose help
reads "diff branch or commit") and `--config <file>`. Without a threshold gremlins exits 0
whatever survives, so `threshold_efficacy` (default 100: every mutant that was run must be killed)
is what makes the step able to fail.

The threshold is passed in a generated configuration file, not as `--threshold-efficacy`:
gremlins v0.6.0 ignores that flag (its flag value reaches the comparison as a string and reads as
0, so no gate applies and a run with 0% efficacy exits 0), but honours `unleash.threshold.efficacy`
from a configuration file. gremlins fails when efficacy is at or below the threshold and treats 0
as no gate, so the file holds `threshold_efficacy` less 0.0001: a run at exactly the threshold
passes, one below it fails. A test with a real gremlins run (skipped when gremlins is not on
`PATH`) proves the gate; the fakes in the other tests only prove the exit code is honoured.

## Recorded gaps (TOOLING-SPEC 9.2)

- Advisory identifiers and `deadcode` have no bundled page, so `go-qa rule-doc GO-...` and
  `go-qa rule-doc deadcode` report them as dangling until their explain routes join the
  resolution order (see `docs/defences.md`).
- `govulncheck` fetches the vulnerability database over the network, so `vuln` cannot run offline;
  an unreachable database is an error, never a pass.
- `govulncheck`, `deadcode` and `gremlins` are pinned in `tools.lock` and installed by
  `scripts/install-tools.sh`, which CI uses. The self-gate still keeps `mutants` off: no mutation
  run of this repository has been held to a threshold.
- The identifiers `error[mod]`, `error[test]`, `error[coverage]` and `error[mutants]` name a runner,
  not a defence, and resolve to no page: `go-qa rule-doc coverage` reports it as dangling. Whether a
  runner verdict is a Defence in the sense of TOOLING-SPEC 4.2 is not settled here, so the gap is
  recorded rather than papered over with pages.
- `--diff <base>` is passed to gremlins as given. gremlins documents it as "diff branch or commit";
  whether it compares with the tip of the ref or with the merge base was not verified, so the
  README does not promise a merge base for `mutants.base`.
- In a `--changed` run, `coverage` applies the module-wide `fail_under_statements` threshold to the
  affected packages only, so a narrowed result can differ from the full run's.
- The `licences` runner of the design brief is not built.
