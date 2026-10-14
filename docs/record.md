# The project record

The project record is the only place an exception to a defence is decided. It lives at
`qa/record.toml` (configurable with `record = "<path>"` in `qa/go-qa.toml`), and go-qa loads it
itself (TOOLING-SPEC 6.1). A missing file is an empty record.

```toml
[ratchet]                      # ceiling on the number of exceptions
high_water = 1
decided_by = "example-owner"
decided_on = 2026-01-15
reason = "why the ceiling is at this value"

[[exception]]
rule = "errcheck"              # identifier exactly as the defence prints it
path = "internal/parse/x.go"   # one file; a glob needs policy = true
justification = "the hazard accepted, and the scope it is accepted in"
decided_by = "example-owner"
added_by_model = "human"       # or the model id that wrote the entry
decided_on = 2026-01-15
review_by = 2026-04-15
policy = false                 # true marks a deliberately broad (glob) exception
covers = "suppressions"        # or "findings"
```

Unknown keys, malformed dates and an unknown `covers` value are errors (exit 2), never ignored.
`covers = "suppressions"` licenses a suppression written in the source or configuration;
`covers = "findings"` licenses the findings a detector that applies the record reports at the path.

## gqaci.record.invalid

`go-qa record check` and the `record` tool report one `gqaci.record.invalid` finding per
defective exception, naming its number, its rule and the line of its `[[exception]]` header, and
listing every problem found in it. An exception is invalid when:

- it has no rule, or no path, or a path that is a malformed glob;
- its path is a glob (`*`, `?`, `[`) and it is not marked `policy = true`;
- it has a `decided_by` that is blank;
- its justification fails the check below (TOOLING-SPEC 6.2).

### The justification check

A justification must name the hazard being accepted and the scope of the exception. go-qa cannot
verify that; it rejects what could be pasted onto any exception unchanged:

- empty text;
- a whole text, compared lower-cased with punctuation trimmed from both ends, that is one of:
  `todo`, `tbd`, `fixme`, `legacy`, `legacy code`, `needed`, `needed for now`, `required`,
  `temporary`, `temp`, `fix later`, `later`, `n/a`, `na`, `none`, `ignore`, `wip`, `hack`,
  `workaround`, `false positive`, `see above`, `as above`;
- a placeholder word anywhere in the text: `todo`, `tbd`, `fixme`, `xxx`;
- fewer than 25 characters.

Passing this check is not evidence that the sentence is true or that it names the right hazard.
That judgement is the owner's, which is why `go-qa record list` puts every justification in one
listing (TOOLING-SPEC 6.3), where a vacuous one is read next to its neighbours.

## gqaci.record.stale

An exception that licenses nothing any defence reports is stale. The record package exposes a
matcher that detectors use to apply the record; it remembers which exceptions were used, and
reports the rest as `gqaci.record.stale`. The `record` tool does not judge staleness by itself,
because that needs every detector that applies the record to have run first. The `suppression` tool
is the only one that applies `covers = "suppressions"` exceptions, so on a run over the whole project
it reports the unused ones as `gqaci.record.stale` (docs/suppression.md). Exceptions with
`covers = "findings"` are applied by the golangci detector and the analyzers detector. The
analyzers detector reports the unused ones for analyzer names on a run over the whole project
(docs/analyzers.md); no run reports an unused findings exception for a golangci-lint linter yet,
which is a recorded gap, not a claim.

## Anti-bloat controls (defaults serving TOOLING-SPEC 6.4)

`go-qa record check` and the `record` tool apply the `[record]` policy from `qa/go-qa.toml`
(`max_total`, `max_per_rule`, `max_policy`, `max_review_days`, `deciders`, `[record.trust]`).
Deadlines are measured against the current UTC day.

### gqaci.record.invalid: provenance and trust

Beyond the structural checks above, an exception is invalid when:

- it lacks `decided_by`, `added_by_model`, `decided_on` or `review_by`;
- its `added_by_model` maps to no tier: `human` is reserved, any other value must start with a
  prefix listed in `[record.trust.models]` (case-insensitive, longest prefix wins);
- it is marked `human` but `decided_by` is not one of `deciders`;
- its rule matches a `[[record.trust.cosign]]` category whose `min_tier` is above the author's
  tier and `decided_by` is not one of `deciders` (the human co-sign);
- `decided_on` is in the future, `review_by` is before `decided_on`, or `review_by` is further
  from `decided_on` than the author tier's horizon: the tier's `max_review_days`, never more than
  the record-wide `max_review_days` (defaults: high 90, medium 30, low 14, human the record-wide
  180);
- `policy = true` is set on a path that names a single file.

### gqaci.record.expired

An exception whose `review_by` is before today fails until it is re-decided (fresh `decided_on`
and `review_by`) or removed. An entry due today still passes.

### gqaci.record.budget

The record fails when it holds more than `max_total` exceptions; more than `max_per_rule` for one
rule (spellings differing in case, surrounding space or `-` against `_` count as one rule); more
than `max_policy` entries marked `policy = true`; more than a tier's `max_count` (never above
`max_total`) written by that tier; or a `[ratchet]` `high_water` above `max_total`.

## The ratchet

`[ratchet]` is the signed ceiling on the number of exceptions (`gqaci.record.ratchet`). It fails when

- the record holds exceptions but has no `[ratchet]` table;
- `decided_by` is not a configured decider, `reason` fails the justification check, or `decided_on`
  is in the future;
- the record holds more exceptions than `high_water`;
- `high_water` is higher than in the record committed at `HEAD` or at the merge base with
  `history_base` (the lower of the two is the reference), unless `decided_on` is newer than the
  previous signature and at most 14 days old. Adopting a ratchet at the existing count is not a
  raise;
- the record shrank and `high_water` is still above the count.

Outside CI, `go-qa run` lowers `high_water` to the count in place and says so; in CI the unlowered
ceiling fails until the lowered ceiling is committed. `go-qa record prune <number>...` removes the
exceptions numbered as `record list` shows them, with the comments above them, and lowers
`high_water` to what remains.

When git or the record's history is unavailable (no repository, no commit, or a record at `HEAD`
that does not parse), a raise cannot be detected, and that is a gap, not a pass: the gate fails with
a `gqaci.record.ratchet` finding in CI (and in `go-qa record check`) and prints a note locally. A
record absent from a commit that exists counts as empty, so creating a record is a raise from zero.
When `history_base` does not resolve to a merge base with `HEAD` (a missing `origin`, a different
default branch, a shallow clone), a raise already committed on the branch cannot be seen either: the
same finding in CI, a note locally. Set `history_base` to a ref that exists, and fetch enough
history for it. `history_base` and `mutants.base` must be refs, never a value starting with `-`.

Dates are whole days, so a second raise signed on the day of the previous signature is accepted
when it is a new decision (another `decided_by` or a different `reason`); a signature that changed
nothing is not fresh. A raise that was signed and committed on a long-lived branch is judged against
the merge base, so once its `decided_on` is older than 14 days CI on that branch fails until the
branch merges or the raise is signed afresh.

The ratchet, the budgets and the tiers are the toolchain's own defaults, offered to serve
TOOLING-SPEC 6.4 (a toolchain should state its defaults); 6.4 does not require them.

## Configuration exceptions

Some findings are configured away in `.golangci.yml` rather than suppressed at the site. Each is
a project-level exception that the suppression detector requires to be recorded in
`qa/record.toml`, with a justification, so that the configuration cannot widen unnoticed. The
recorded rule is the one the detector prints (docs/suppression.md), and the record entry's path is
the configuration file.

| Detector rule          | Scope                                           | Hazard                                                                         | Why it does not apply                                                                                                   |
| ---------------------- | ----------------------------------------------- | ------------------------------------------------------------------------------ | ----------------------------------------------------------------------------------------------------------------------- |
| gosec `G204`           | whole module                                    | command injection through a shell or a variable program name                   | every launch is an argument vector, never a shell; program names are constants or resolved tool paths                   |
| gosec `G306`           | `tests/scripts/secret_scan_test.go` only        | a file written more permissive than 0600 can be read or altered by other users | the file is a test stub that must be executable (0700, owner only), written into a private per-test temporary directory |
| govet `fieldalignment` | whole module (disabled in the bundled baseline) | padding bytes wasted in memory-heavy structs                                   | struct layout is not a defect class this toolchain defends against, and the module holds no memory-heavy struct         |

The entries are `golangci.yml::gosec.excludes::G204`, `golangci.yml::exclusions.rules::gosec/G306`
and `golangci.yml::govet.disable::fieldalignment`, all against `.golangci.yml`. Their review dates
are the owner's to re-decide; a lapsed one fails `gqaci.record.expired`. The `example-owner`
decider in `qa/go-qa.toml` is a placeholder to replace before the first public release.

## Order of checks

The matcher that detectors use to apply the record also matches an entry that validation rejects
(a glob without `policy = true`, say). That is deliberate and relies on an ordering: the `record`
tool is a detector and fails the run with `gqaci.record.invalid` before any runner starts, so an
invalid entry never licenses anything in a green run. A run that names tools with `-t` still
includes the `record` tool whenever it is switched on and any tool other than `fmt` is named; with
`record = false` in the configuration the entries are not validated on any run. An entry whose `added_by_model` maps to no tier
is likewise invalid, and counts toward no tier's `max_count` while being reviewed against the lowest
tier's horizon.

## Not yet built

`record prune` takes explicit numbers; it does not yet find the stale exceptions itself. The
`suppression` tool reports the stale suppression exceptions, and `record prune` removes them.
