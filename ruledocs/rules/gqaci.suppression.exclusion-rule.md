# gqaci.suppression.exclusion-rule

## What it flags

An entry in `linters.exclusions.rules` of a golangci-lint configuration
(`.golangci.{yml,yaml,toml,json}`): a rule that drops the findings of some linters, or of findings
whose text matches, in the files a `path` or `source` pattern selects. The finding names the
entry as `golangci.yml::exclusions.rules::<linters>[/<text>]` and points at its line.

## Why

A check either runs on all code or is off for all code with its reason stated. An exclusion rule
does neither: it keeps the check in the defence set while hiding it wherever the pattern reaches,
and a list of such rules is a baseline of findings nobody fixed. The same decision can always be
made in one of the two honest forms, so no record entry can license an exclusion rule, and an
exception in the project record that names one is itself invalid (`gqaci.record.invalid`) and,
being unused, stale (`gqaci.record.stale`).

## How to fix correctly

Fix the code so the check passes, and delete the exclusion rule. If the check is more noise than
signal for this project, turn it off for all code instead, with a one-line reason as a comment on
the line: a linter in `linters.disable`, or the rule in its linter's own switch-off list
(`gosec.excludes`, `govet.disable`, `gocritic.disabled-checks`, `testifylint.disable`, a staticcheck
`-XXXX` check, a revive rule with `disabled: true`). A switch-off for all code needs no record
entry. Then delete any record entry that named the exclusion rule.

## Suppression

Not suppressible, and no record entry can license it.
