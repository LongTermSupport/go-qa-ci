# gqaci.suppression.unrecorded

## What it flags

A suppression with no matching exception in the project record: a nolint, `#nosec`, ast-grep
ignore or staticcheck `lint:ignore` directive; a hand-written generated-code header; a
`staticcheck.conf` that removes or whitelists checks; or an exclusion or disabled rule in
`.golangci.yml`, whose rule and path no record entry licenses. A Go or configuration file that cannot
be parsed is reported under this identifier too, because what it silences cannot be verified.

## Why

A suppression written in the source is invisible to review and cannot be counted, dated or bounded.
Routing every suppression through the record keeps the exceptions few, justified and expiring.

## How to fix correctly

Fix the code so the finding disappears, and delete the directive. Only where the finding is a true
false positive for this project, add an exception to the record that names the rule and the exact
path, with a justification stating the hazard and why it does not apply. The finding prints the rule
and path to record. An unparseable file is fixed, not recorded.

## Suppression

Not suppressible except through the record entry this finding asks for.
