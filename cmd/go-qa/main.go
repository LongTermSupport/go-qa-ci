// Command go-qa is the Defence Before Fix toolchain entry point for Go modules.
package main

import (
	"fmt"
	"io"
	"os"
)

const version = "0.0.0-dev"

const (
	exitOK      = 0
	exitFailure = 1
	exitUsage   = 2
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// emit writes one formatted line to w and returns code, or exitFailure when the write fails.
func emit(w io.Writer, code int, format string, args ...any) int {
	if _, err := fmt.Fprintf(w, format+"\n", args...); err != nil {
		return exitFailure
	}
	return code
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return emit(stderr, exitUsage, "usage: go-qa --version")
	}
	if args[0] == "--version" {
		return emit(stdout, exitOK, "%s", version)
	}
	return emit(stderr, exitUsage, "go-qa: unknown subcommand %q", args[0])
}
