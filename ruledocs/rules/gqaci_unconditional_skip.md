# gqaci_unconditional_skip

## What it flags

A call of `Skip`, `Skipf` or `SkipNow` of the `testing` package that is a statement at the top
level of the body of a `Test`, `Benchmark` or `Fuzz` function, or of a function literal passed to
`Run`, in a `_test.go` file. A skip inside an `if`, a `switch` or any other nested statement is
conditional and is not flagged, and neither is one in a helper function.

## Why

Everything after such a skip is dead code, and the test counts as skipped, which a green run
reports without complaint. The behaviour the test was written to defend is no longer defended, and
nobody is told. It is the cheapest way to turn a failing test off and the hardest to notice later.

## How to fix correctly

Fix the test or delete it. If it can only run in some environments, say which, in code:

```go
func TestNeedsDatabase(t *testing.T) {
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("DATABASE_URL is not set")
	}
	// ...
}
```

## Limits

A skip placed after an unconditional `return`, or one reached through a helper that always skips,
is not recognised.

## Suppression

Blocking. The analyzer has no inline suppression route; the only exception is a project-record
entry with `covers = "findings"` and the rule `gqaci_unconditional_skip`, which names the path it licenses and gives
the justification.
