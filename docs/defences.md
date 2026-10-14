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
the project stage of that name or the documentation its rules command ships for the rule
(`docs/stages.md`), then bundled page, then the documentation of a registered analyzer
(`docs/analyzers.md`), then the first configured `[[resolver]]` whose glob matches. A project page therefore overrides a bundled one; an unreadable project page is an
error, never a silent fall-through. Nothing is fetched from the network.

## What a page contains

The first line is `# <id>`. A `## How to fix correctly` section states, specifically enough to
act on, what to write instead. The bundled pages additionally carry "What it flags", "Why" and
"Suppression" sections; project pages should too.

## The built-in identifiers

`gqaci.record.{invalid,stale,budget,ratchet,expired}`, `gqaci.suppression.{unrecorded,bare}`,
`gqaci.summary.stale`, `gqaci.docs.{dangling,orphan}`, `gqaci.rules.{unnamed,untested}` and the
bundled analyzer `gqaci_sensitive_format`. The
list lives in `internal/builtin`; some are raised by detectors that are not built yet, and their
pages exist ahead of them so that the release audit (below) never has a gap to excuse.

## The docs detector (`docs` tool, `go-qa docs-check`)

- `gqaci.docs.dangling`: a built-in identifier, a project stage name, a rule a stage prints or a
  rule of the project's ast-grep rule directories (read from `sgconfig.yml`, so ast-grep need not
  be installed; skipped when the `ast-grep` tool is off) has no page, or a bundled page does not begin with `# <id>`, has no non-empty `## How to fix correctly`
  section, or belongs to no built-in (TOOLING-SPEC 8.1: a Go test and this check fail CI's `go` and
  `self-gate` jobs on every push; no release process yet refuses to publish a tag on a red run, a
  recorded gap in `docs/CONFORMANCE.md`).
- `gqaci.docs.orphan`: a page in `docs_dir` documents no built-in identifier, no project stage or
  rule it prints, no project ast-grep rule, no analyzer of a configured project vet driver and no
  rule in the project record. `README.md` is exempt.

## Recorded gaps

These are gaps, not claims (TOOLING-SPEC 9.2):

- Identifiers of third-party detectors (golangci-lint linters, staticcheck checks) do not resolve
  yet; analyzers do, through the driver's `help`. The `golangci` detector prints them (`errcheck`, `staticcheck::SA1019`) but
  `go-qa rule-doc` has no page for them; they join the resolution order through their own explain
  routes (TOOLING-SPEC 4.2). `docs/golangci.md` records the gap per clause.
- The detector enumerates the identifiers of built-ins, project stages and their rules, project
  ast-grep rules and, for a configured project vet driver, its analyzers (a page for one is
  allowed, not required, because the driver's `help` documents it). It does not enumerate
  golangci-lint linters, so a project page named for one is reported as an orphan. The bundled
  `gqaci.ast.*` pack rules are neither enumerated nor given pages, so `go-qa rule-doc` reports
  them dangling and `go-qa docs-check` does not.
- Project pages are checked for existence and naming, not for the shape of their content. A
  project page named like a bundled identifier replaces the bundled remediation, so an empty
  `gqaci.record.stale.md` in `docs_dir` is accepted; nothing audits a page that shadows a built-in.
- Only regular `.md` files count as pages when orphans are listed; a symlinked page is never an
  orphan, although `rule-doc` reads it under its exact name.
- When the bundled `gqaci.ast.*` pack gains rules, their identifiers must join `builtin.All()` (or
  the 8.1 audit must enumerate them, with the family patterns of DETECTOR-SPEC 6.5); until they do,
  the audit passes without covering them.
- Advisory identifiers (`GO-YYYY-NNNN`, from `vuln`) and `deadcode` have no page yet; see
  `docs/runners.md`.
