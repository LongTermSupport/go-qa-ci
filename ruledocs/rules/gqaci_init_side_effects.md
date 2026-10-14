# gqaci_init_side_effects

## What it flags

In the body of an `init` function: a `go` statement, and a call of a function or method that does
input or output or changes the process. That is anything in `os` except the readers of the
environment and of the process identity (`Getenv`, `LookupEnv`, `Environ`, `ExpandEnv`, `Getpid`,
`UserHomeDir` and similar), `os/exec`, `os/signal`, `os/user`, `io/ioutil`, `syscall`, `net`,
`net/rpc`, `net/smtp`, `database/sql` (except `Register` and `Drivers`), the client and server of
`net/http`, `fmt.Print`, `fmt.Printf`, `fmt.Println`, `flag.Parse`, and `time.Sleep` and the
timers. A function literal called at once counts; one that is only stored does not.

Assigning package variables, `regexp.MustCompile`, building lookup tables, `sql.Register` and
registering a handler are the legitimate work of `init` and are not flagged.

## Why

`init` runs at import, in an order nobody chose, before `main` and before every test, in every
binary that links the package, including tools that only wanted a type from it. Input or output
there cannot be skipped, has no caller to report its error to, makes the package unsafe to import
and impossible to test without running it, and a goroutine started there outlives any shutdown
sequence.

## How to fix correctly

Move the work into a function and call it from `main` (or from the code that needs it), where its
error is handled:

```go
func Setup(dir string) error {
	return os.MkdirAll(dir, 0o700)
}
```

## Limits

Only the body of `init` is read. A side effect behind a call to a function of the same package is
not followed, and neither is package-level variable initialisation such as
`var conn = connect()`.

## Suppression

Blocking. The analyzer has no inline suppression route; the only exception is a project-record
entry with `covers = "findings"` and the rule `gqaci_init_side_effects`, which names the path it licenses and gives
the justification.
