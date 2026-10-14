# gqaci.suppression.bare

## What it flags

A suppression that names no rule, such as a nolint directive with no linter list, a gosec marker
with no code or an ast-grep ignore with no rule id; or one that gives no reason, such as a nolint
directive naming a linter but no `// reason`. It silences every finding on its line, including ones
that do not exist yet.

## Why

A bare suppression cannot be matched to a record entry, so no decision can license it. It also hides
whatever the next change introduces on that line.

## How to fix correctly

Remove the directive and fix the code. If a single named rule genuinely does not apply, name it in
the directive and record the exception.

## Suppression

Not suppressible, and no record entry can license it.
