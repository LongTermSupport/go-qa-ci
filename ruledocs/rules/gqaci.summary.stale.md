# gqaci.summary.stale

## What it flags

The generated agent summary region in the configured `[summary] file` differs from what
`go-qa summary` would write for the current configuration.

## Why

The summary is what an agent reads about the active defences. A stale one describes defences that no
longer run, or omits ones that do.

## How to fix correctly

Run `go-qa summary` and commit the regenerated region. Do not edit the region by hand: it is
overwritten.

## Suppression

Not suppressible. Regenerate it.
