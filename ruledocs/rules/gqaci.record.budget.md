# gqaci.record.budget

## What it flags

A project record that is over a budget set in `[record]` of `qa/go-qa.toml`: more than `max_total`
exceptions, more than `max_per_rule` for one rule (spellings differing in case, space, or `-`
against `_` count as one rule), more than `max_policy` broad entries marked `policy = true`, more
than a trust tier's `max_count`, or a ratchet `high_water` above `max_total`.

## Why

A record that only grows stops being a list of deliberate decisions and becomes a way of making the
gate pass. The budgets keep every entry reviewable.

## How to fix correctly

Remove exceptions by fixing the code they excuse, then prune them from the record. Do not raise a
budget to make the gate pass: raising one is a decision for the owner, made in `qa/go-qa.toml` and
signed through the ratchet.

## Suppression

Not suppressible. Reduce the record.
