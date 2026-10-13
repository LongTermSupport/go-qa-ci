# gqaci.record.ratchet

## What it flags

A problem with the `[ratchet]` table of the project record, the signed ceiling on the number of
exceptions: the record holds exceptions but has no ratchet; `decided_by` is not a configured
decider; `reason` is generic; `decided_on` is in the future; the record holds more exceptions than
`high_water`; `high_water` is higher than in the record committed at `HEAD` or at the merge base
with `history_base` without a fresh signed decision; the record shrank and the ceiling is still
above the count; or `history_base` does not resolve, so a raise cannot be checked (in CI).

## Why

Without a ceiling, exceptions accumulate one reasonable-looking entry at a time. The ceiling only
moves down unless a configured decider signs a raise.

## How to fix correctly

If the record shrank, lower `high_water` to the count (outside CI `go-qa run` does it for you; in
CI commit the lowered value). To raise the ceiling, a configured decider updates `decided_by`,
sets `decided_on` to today and writes a `reason` naming why the raise is needed. If `history_base`
does not resolve, set it to a ref that exists and fetch enough history for it.

## Suppression

Not suppressible. Correct the ratchet or the record.
