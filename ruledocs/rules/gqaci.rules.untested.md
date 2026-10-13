# gqaci.rules.untested

## What it flags

A project ast-grep rule that has no test case with both valid code (the rule must stay silent) and
invalid code (the rule must report).

## Why

A rule without a red proof cannot be shown to catch what it claims to, and one without a green proof
cannot be shown not to flag correct code.

## How to fix correctly

Add a test file for the rule listing at least one `valid` and one `invalid` snippet, and run
`ast-grep test` until it passes.

## Suppression

Not suppressible. Add the cases.
