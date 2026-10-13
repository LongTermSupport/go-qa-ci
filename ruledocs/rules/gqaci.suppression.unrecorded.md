# gqaci.suppression.unrecorded

## What it flags

A suppression with no matching exception in the project record: a `//nolint`, `#nosec` or
`lint:ignore` directive, or an exclusion or disabled rule in `.golangci.yml`, whose rule and path
no record entry licenses.

## Why

A suppression written in the source is invisible to review and cannot be counted, dated or bounded.
Routing every suppression through the record keeps the exceptions few, justified and expiring.

## How to fix correctly

Fix the code so the finding disappears, and delete the directive. Only where the finding is a true
false positive for this project, add an exception to the record that names the rule and the exact
path, with a justification stating the hazard and why it does not apply.

## Suppression

Not suppressible except through the record entry this finding asks for.
