# gqaci_shell_exec

## What it flags

A call of `exec.Command`, `exec.CommandContext`, `os.StartProcess` or `syscall.Exec` whose program
is, by constant analysis (a literal, a named constant or a constant expression), a shell (`sh`,
`bash`, `zsh`, `dash`, `ash`, `ksh`, `csh`, `tcsh`, `fish`, with or without a directory), directly
or behind a wrapper program such as `env`, `sudo`, `nohup` or `timeout`, and whose leading options
include `-c` or a cluster of short options ending in `c` such as `-lc`. For `os.StartProcess` and
`syscall.Exec` the vector must be written as a slice literal in the call.

## Why

`sh -c "..."` puts the program and its arguments into a single string that the shell parses again.
Quoting becomes the caller's job, and a value holding a space, a quote, `;`, `$` or a backtick
changes the command. Every control keyed on the argument vector (a command allowlist, a security
linter's call-site check, a sandbox policy) sees only `sh`, so the shell wrapper is the standard way
to get a command past them.

## How to fix correctly

Pass the program and each argument as separate values, so nothing is parsed:

```go
cmd := exec.CommandContext(ctx, "git", "-C", dir, "status", "--short")
```

If a pipeline is needed, start both processes and connect the first one's `StdoutPipe` to the
second one's `Stdin`.

## Limits

A program or option that is not constant, or arguments spread from a slice (`args...`), are not
decidable and are not flagged. A shell started to run a script file (`bash script.sh`) is not
flagged, because its argument is not a command string.

## Suppression

Blocking. The analyzer has no inline suppression route; the only exception is a project-record
entry with `covers = "findings"` and the rule `gqaci_shell_exec`, which names the path it licenses and gives
the justification.
