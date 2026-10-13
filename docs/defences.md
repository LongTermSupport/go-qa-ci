# Identifiers and their pages

Every identifier go-qa prints resolves offline to a page that states the correct construction,
not only the prohibition (TOOLING-SPEC 4.2, DBF method clause 8.4). `go-qa rule-doc <id>` prints
the page; an identifier with none is dangling and exits 1.

## Where pages live

| Pages   | Location                                | Written by        |
| ------- | --------------------------------------- | ----------------- |
| Bundled | `ruledocs/rules/<id>.md`, embedded      | go-qa             |
| Project | `docs_dir` (`docs/defences`), `<id>.md` | the rule's author |

A project page is named after the identifier exactly as printed. A page whose name differs only in
case, or in `-` against `_`, documents the same identifier. Resolution order: project page, then
bundled page. A project page therefore overrides a bundled one; an unreadable project page is an
error, never a silent fall-through. Nothing is fetched from the network.

## What a page contains

The first line is `# <id>`. A `## How to fix correctly` section states, specifically enough to
act on, what to write instead. The bundled pages additionally carry "What it flags", "Why" and
"Suppression" sections; project pages should too.

## The built-in identifiers

`gqaci.record.{invalid,stale,budget,ratchet,expired}`, `gqaci.suppression.{unrecorded,bare}`,
`gqaci.summary.stale`, `gqaci.docs.{dangling,orphan}` and `gqaci.rules.{unnamed,untested}`. The
list lives in `internal/builtin`; some are raised by detectors that are not built yet, and their
pages exist ahead of them so that the release audit (below) never has a gap to excuse.

## The docs detector (`docs` tool, `go-qa docs-check`)

- `gqaci.docs.dangling`: a built-in identifier has no page, or a bundled page does not begin with
  `# <id>`, has no non-empty `## How to fix correctly` section, or belongs to no built-in
  (TOOLING-SPEC 8.1: a Go test and this check, which the self-gate runs, fail the release).
- `gqaci.docs.orphan`: a page in `docs_dir` documents no built-in identifier and no rule in the
  project record. `README.md` is exempt.

## Recorded gaps

These are gaps, not claims (TOOLING-SPEC 9.2):

- Identifiers of third-party detectors (golangci-lint linters, staticcheck checks, vet analyzers)
  do not resolve yet; they join the resolution order with the detectors that run them, through
  their own explain routes (TOOLING-SPEC 4.2).
- The defence listing (`rules`, M11) does not exist, so the detector cannot require a page for a
  project rule it cannot enumerate, and treats only built-ins and rules named in the project
  record as identifiers the project can see. A page for any other project rule is reported as an
  orphan until the listing feeds the detector.
- Project pages are checked for existence and naming, not for the shape of their content.
