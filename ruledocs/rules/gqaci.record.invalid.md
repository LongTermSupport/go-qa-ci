# gqaci.record.invalid

## What it flags

An exception in the project record (`qa/record.toml`) that is malformed or insufficiently
justified: no rule or path, a glob path without `policy = true`, a missing or blank `decided_by`,
`added_by_model`, `decided_on` or `review_by`, a model that maps to no trust tier, a human entry
signed by someone who is not a configured decider, a review date outside the tier's horizon, or a
justification that is empty, shorter than 25 characters, a bare word such as `legacy`, or carries a
placeholder (`todo`, `tbd`, `fixme`, `xxx`). Each finding names the entry's number, its rule and
the line of its `[[exception]]` header.

## Why

The record is the only place an exception is decided. An exception nobody can attribute, date or
justify is a hole that cannot be reviewed.

## How to fix correctly

Edit the entry so that it states a decision. Name the hazard the rule guards against, say why it
does not apply here, and give the scope (one file, or the glob and why it is that wide). Fill in
`decided_by`, `added_by_model` (`human` or a model id with a configured prefix), `decided_on` and a
`review_by` inside the horizon. If you cannot write an honest justification, delete the entry and
fix the code the rule flags instead.

## Suppression

Not suppressible. A record entry cannot excuse a defect in the record.
