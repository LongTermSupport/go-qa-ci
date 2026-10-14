package astgrep

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// Binary is the executable name. It is deliberately not `sg`, which on Linux is a different
// program (newgrp).
const Binary = "ast-grep"

// ErrNotInstalled is wrapped by the error Locate returns when the binary is not on PATH.
var ErrNotInstalled = errors.New(Binary + " is not installed")

// Runner runs one ast-grep binary.
type Runner struct {
	// Path is the path of the ast-grep executable.
	Path string
}

// Locate finds ast-grep on PATH.
func Locate() (Runner, error) {
	path, err := exec.LookPath(Binary)
	if err != nil {
		return Runner{}, fmt.Errorf("%w: %s was not found on PATH", ErrNotInstalled, Binary)
	}
	return Runner{Path: path}, nil
}

// ScanOptions select what a scan runs and over what.
type ScanOptions struct {
	// Rule is a rule file to scan with instead of the project's sgconfig.yml rules.
	Rule string
	// Excludes are globs of files the scan leaves out.
	Excludes []string
	// Paths limit the scan; empty means the whole directory.
	Paths []string
}

// Scan runs `ast-grep scan` in dir and returns its matches. Exit status 1 only means that
// error-severity rules matched; any other failure, or output that is not a match list, means the
// scan did not run.
func (r Runner) Scan(ctx context.Context, dir string, options ScanOptions) ([]Match, error) {
	args := []string{"scan", "--json=compact"}
	if options.Rule != "" {
		args = append(args, "--rule", options.Rule)
	}
	for _, glob := range options.Excludes {
		args = append(args, "--globs", "!"+glob)
	}
	args = append(args, options.Paths...)
	stdout, stderr, code, err := r.run(ctx, dir, args)
	if err != nil {
		return nil, err
	}
	if code != 0 && code != 1 {
		return nil, fmt.Errorf("`%s %s` exited with status %d: %s", Binary, strings.Join(args, " "), code, strings.TrimSpace(stderr))
	}
	matches, err := ParseMatches([]byte(stdout))
	if err != nil {
		return nil, fmt.Errorf("`%s %s`: %w", Binary, strings.Join(args, " "), err)
	}
	for i := range matches {
		matches[i].Path = filepath.ToSlash(filepath.Clean(matches[i].Path))
	}
	return matches, nil
}

// Test runs the project's rule tests (`ast-grep test`, without snapshot tests, which need an
// interactive review) in dir. It returns the combined output and whether they all passed.
func (r Runner) Test(ctx context.Context, dir string) (output string, passed bool, err error) {
	stdout, stderr, code, err := r.run(ctx, dir, []string{"test", "--skip-snapshot-tests"})
	if err != nil {
		return "", false, err
	}
	return strings.TrimSpace(stdout + stderr), code == 0, nil
}

// run executes the binary in dir; err is set only when it could not be run at all.
func (r Runner) run(ctx context.Context, dir string, args []string) (stdout, stderr string, code int, err error) {
	var out, errOut bytes.Buffer
	cmd := exec.CommandContext(ctx, r.Path, args...)
	cmd.Dir = dir
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	runErr := cmd.Run()
	var exitErr *exec.ExitError
	switch {
	case runErr == nil:
		return out.String(), errOut.String(), 0, nil
	case errors.As(runErr, &exitErr):
		return out.String(), errOut.String(), exitErr.ExitCode(), nil
	default:
		return "", "", 0, fmt.Errorf("running %s: %w", Binary, runErr)
	}
}
