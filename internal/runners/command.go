// Package runners launches the external programs behind the runner tools (vuln, mod, test,
// coverage, mutants) and the deadcode detector, and turns their output into findings or a verdict.
//
// A required program that is not installed is an error, never a skipped check.
package runners

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
)

// Output is what a finished program printed and how it exited.
type Output struct {
	Stdout string
	Stderr string
	Code   int
}

// Combined is stdout followed by stderr.
func (o Output) Combined() string {
	return o.Stdout + o.Stderr
}

// Locate finds a required program on PATH.
func Locate(binary string) error {
	if _, err := exec.LookPath(binary); err != nil {
		return fmt.Errorf("%s not found on PATH: install the pinned version; it is a required tool and its absence is never a pass", binary)
	}
	return nil
}

// Capture runs name in dir and returns its output and exit code; err is set only when the process
// could not be run at all.
func Capture(ctx context.Context, dir, name string, args ...string) (Output, error) {
	var out, errOut bytes.Buffer
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	runErr := cmd.Run()
	var exitErr *exec.ExitError
	switch {
	case runErr == nil:
		return Output{Stdout: out.String(), Stderr: errOut.String()}, nil
	case errors.As(runErr, &exitErr):
		return Output{Stdout: out.String(), Stderr: errOut.String(), Code: exitErr.ExitCode()}, nil
	default:
		return Output{}, fmt.Errorf("running %s: %w", name, runErr)
	}
}
