# gqaci.suppression.bare

## What it flags

A suppression that names no rule, such as a `//nolint` with no linter list. It silences every
finding on its line, including ones that do not exist yet.

## Why

A bare suppression cannot be matched to a record entry, so no decision can license it. It also hides
whatever the next change introduces on that line.

## How to fix correctly

Remove the directive and fix the code. If a single named rule genuinely does not apply, name it in
the directive and record the exception.

## Suppression

Not suppressible, and no record entry can license it.
