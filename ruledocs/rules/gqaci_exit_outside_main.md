# gqaci_exit_outside_main

## What it flags

Any use, as a call, a method value or a function stored in a variable, of `os.Exit`,
`syscall.Exit`, `log.Fatal`, `log.Fatalf`, `log.Fatalln` or the same three methods of
`*log.Logger`, anywhere except inside the function `main` of package `main`, or `TestMain` (the
function taking a `*testing.M`) of a test file. Function literals inside `main` count as inside it.

## Why

Exiting does not unwind: no deferred call runs, in any goroutine, so buffers stay unflushed,
temporary files stay behind and locks stay held. The caller cannot recover or intercept it, and a
test that reaches it terminates the whole test binary instead of failing. A library that exits
takes the decision of how the program ends away from its owner.

## How to fix correctly

Return the error, and let `main` print it and choose the exit code:

```go
func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

## Limits

`panic` and `testing`'s `FailNow` are different mechanisms and are not judged. A function named
`main` in a package that is not `main` is not the entry point and is flagged.

## Suppression

Blocking. The analyzer has no inline suppression route; the only exception is a project-record
entry with `covers = "findings"` and the rule `gqaci_exit_outside_main`, which names the path it licenses and gives
the justification.
