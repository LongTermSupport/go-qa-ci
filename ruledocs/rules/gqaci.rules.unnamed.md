# gqaci.rules.unnamed

## What it flags

A project ast-grep rule with no `id`.

## Why

Without an identifier a finding cannot be recorded, documented or counted, and the message points
nowhere.

## How to fix correctly

Give the rule an `id` that is stable and meaningful (not derived from its position), and add a page
named after it in the project documentation directory.

## Suppression

Not suppressible. Name the rule.
