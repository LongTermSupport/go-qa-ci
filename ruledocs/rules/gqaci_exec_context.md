# gqaci_exec_context

## What it flags

Any use of `os/exec.Command`, as a call, a method value or a function stored in a variable, at a
place where a context is in scope: a parameter, a receiver or a local variable declared before the
use, in the function or in one that encloses it, whose type is `context.Context`, `*testing.T`,
`*testing.B`, `*testing.F` or `testing.TB`. The match is on the resolved function, so an import
alias or a dot import does not escape it.

## Why

A process started with `exec.Command` knows nothing of the context. When the caller cancels or its
deadline passes, the call site stops waiting but the child keeps running, and a hung child holds
the call, its pipes and its descriptors for ever. A function that was given a context was asked to
honour it.

## How to fix correctly

Use `exec.CommandContext`, with the context in scope:

```go
func list(ctx context.Context, dir string) ([]byte, error) {
	return exec.CommandContext(ctx, "ls", dir).Output()
}
```

In a test, use `t.Context()` (or a context derived from it).

## Limits

A function with no context in scope is not flagged: giving it one is a design change this rule
cannot demand. A context reached through a struct field, a global or a call result is not seen.
`syscall.Exec` and `os.StartProcess` take no context and are not judged.

## Suppression

Blocking. The analyzer has no inline suppression route; the only exception is a project-record
entry with `covers = "findings"` and the rule `gqaci_exec_context`, which names the path it licenses and gives
the justification.
