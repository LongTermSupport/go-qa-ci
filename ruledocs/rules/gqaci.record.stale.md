# gqaci.record.stale

## What it flags

An exception in the project record that licenses nothing: no suppression in the source or
configuration, and no finding reported by a defence, matches its rule and path.

## Why

A stale exception is a pre-approved hole. The next suppression that happens to match it passes
without anyone deciding it. Exceptions must disappear with the code that needed them.

## How to fix correctly

Delete the exception from the record (`go-qa record prune <number>` removes it and lowers the
ratchet ceiling). If the suppression moved, update `path` to its new location and re-read the
justification to confirm it still holds.

## Suppression

Not suppressible. Remove or correct the record entry.
