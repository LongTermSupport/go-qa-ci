# gqaci_defer_in_loop

## What it flags

A `defer` statement inside the body of a `for` or `range` loop of the same function. A function
literal starts a new function, so a `defer` inside a closure that the loop calls is not flagged.

## Why

A deferred call runs when the function returns, not at the end of the iteration. In a loop the
calls pile up, one per iteration: files, locks, connections and transactions stay open until the
loop is over, and a loop over many items exhausts descriptors or holds a lock far longer than
intended. The mistake reads correctly and passes every small test.

## How to fix correctly

Give each iteration its own function, so it returns every time:

```go
for _, path := range paths {
	if err := process(path); err != nil {
		return err
	}
}

func process(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return use(f)
}
```

A function literal called at once (`func() { defer ...; ... }()`) does the same in place.

## Limits

A loop made with `goto` is not recognised as a loop.

## Suppression

Blocking. The analyzer has no inline suppression route; the only exception is a project-record
entry with `covers = "findings"` and the rule `gqaci_defer_in_loop`, which names the path it licenses and gives
the justification.
