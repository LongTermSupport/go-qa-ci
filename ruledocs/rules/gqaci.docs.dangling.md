# gqaci.docs.dangling

## What it flags

An identifier that resolves to no documentation page, or a bundled page that is malformed: its first
line is not `# <identifier>`, it has no `## How to fix correctly` section, or it is not paired with
a built-in identifier. `go-qa docs-check` reports the same.

## Why

An identifier that leads nowhere names a pattern without saying what to do about it. The page must
state the correct construction, not only the prohibition.

## How to fix correctly

Add the page. For a project rule, create `<docs_dir>/<identifier>.md` (`docs/defences` by default)
whose first line is `# <identifier>` and which has a `## How to fix correctly` section stating,
specifically enough to act on, what to write instead. For a bundled page, correct the first line or
add the section in `ruledocs/rules/`.

## Suppression

Not suppressible. Write the page.
