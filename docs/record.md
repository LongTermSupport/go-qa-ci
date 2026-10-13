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
because that needs every detector that applies the record to have run first. Until those detectors
exist (suppression guard, golangci, analyzers) no run reports `gqaci.record.stale`; this is a
recorded gap, not a claim.

## Anti-bloat controls (TOOLING-SPEC 6.4)

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

## Not yet built

The ratchet comparison (lowering, raising against git history, signatures) is separate work. The
`[ratchet]` table is parsed and its `high_water` is checked against `max_total`, but nothing else
enforces it yet, so the record is not yet protected against a rewritten ceiling.
