# gqaci.record.expired

## What it flags

A record exception whose `review_by` date is before today (UTC). An entry due today still passes.
An exception marked `permanent = true` has no review date and is never flagged.

## Why

An exception is a decision made at a point in time, against the code of that time. The review date
forces the decision to be taken again instead of being inherited.

## How to fix correctly

Re-read the justification against today's code. If the hazard is still accepted, re-decide the
entry: set a fresh `decided_on` and a `review_by` inside the horizon of the author's tier. If the
code no longer needs it, delete the entry. Better still, fix the code the rule flags and remove the
exception.

Mark the entry `permanent = true` (dropping `review_by`) only when the exception is inherent to the
code and will never be outgrown, with the justification saying why, and only as a configured
decider: renewing a date by making it permanent to silence this finding is not a fix.

## Suppression

Not suppressible. Re-decide or remove the entry.
