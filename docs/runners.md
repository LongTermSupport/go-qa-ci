# Runners and the deadcode detector

The runners execute code or audit dependencies, after every format and detector step has passed
(TOOLING-SPEC 4.5). They stop at the first failing runner, so a failing `test` keeps `coverage`
and `mutants` from running. Each runs once per Go module the configuration covers (the root module
and `modules`). A required program that is not on `PATH` is an error, never a skipped check.

| Tool | Command | Fails when |
| ---- | ------- | ---------- |
| `vuln` | `govulncheck -format json ./...` | the code calls a vulnerable symbol the record does not license |
| `mod` | `go mod tidy -diff`, then `go mod verify` | `go.mod` or `go.sum` is not tidy, or a dependency does not match its recorded hash |
| `test` | `go test -race -shuffle=on -count=1 ./...` | a test fails or the code does not build |
| `coverage` | `go test -covermode=atomic -coverpkg=./... -coverprofile`, then `go tool cover -func` | statement coverage is below `coverage.fail_under_statements` |
| `mutants` | `gremlins unleash --threshold-efficacy N` (diff mode adds `--diff <base>`) | the tests kill fewer than `mutants.threshold_efficacy` percent of the mutants |
| `deadcode` (detector) | `deadcode -test -json ./...` | a function is unreachable from every `main` function and test |

## Identifiers

- `vuln` prints each finding under the advisory identifier (`GO-YYYY-NNNN`). Scanning is at symbol
  level: a vulnerability in a dependency, or in an imported but never called package, is not a
  finding, because reachability decides. The location is the call site in the module. A record
  exception (`covers = "findings"`) licenses an advisory for a path; on a run over the whole
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
reads "diff branch or commit") and `--threshold-efficacy`. Without a threshold gremlins exits 0
whatever survives, so `threshold_efficacy` (default 100) is what makes the step able to fail.

## Recorded gaps (TOOLING-SPEC 9.2)

- Advisory identifiers and `deadcode` have no bundled page, so `go-qa rule-doc GO-...` and
  `go-qa rule-doc deadcode` report them as dangling until their explain routes join the
  resolution order (see `docs/defences.md`).
- `govulncheck` fetches the vulnerability database over the network, so `vuln` cannot run offline;
  an unreachable database is an error, never a pass.
- The pinned versions of `govulncheck`, `deadcode` and `gremlins` are not recorded in a tool lock
  yet. CI installs `govulncheck` and `deadcode` at fixed versions; `gremlins` is not installed in
  CI, so the self-gate keeps `mutants` off.
- The `licences` runner of the design brief is not built.
