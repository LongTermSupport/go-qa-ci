// Package launch is the one place go-qa's code starts another program, so the hazard of command
// injection through a shell or an attacker-chosen program is answered once, here. (gosec G204 is
// off for all code: go-qa is a tool runner, and gqaci_shell_exec forbids a shell with -c.)
//
// The contract of every launch:
//
//   - an argument vector only: each argument reaches the program as one value, and no shell parses
//     it on go-qa's behalf. A shell is an ordinary program: a project may name one in its own
//     committed stage configuration, as it may name any program, and go-qa's own code never does
//     (the gqaci_shell_exec analyzer holds that);
//   - the program is a bare name found on PATH with exec.LookPath (a constant in go-qa's code, or the
//     program a project names in its own committed configuration), or an absolute path the caller
//     has already resolved. A relative path is refused: it would name a different file from every
//     directory the process is started in;
//   - bound to a context (exec.CommandContext), so cancellation and deadlines stop the process.
package launch

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// Find resolves a bare program name on PATH to an absolute path. A name with a path separator is
// refused: it is not looked up on PATH, so it is not a name. A program that is not installed is an
// error that wraps exec.ErrNotFound.
func Find(name string) (string, error) {
	switch {
	case strings.TrimSpace(name) == "":
		return "", errors.New("no program named")
	case strings.ContainsRune(name, '/') || strings.ContainsRune(name, filepath.Separator):
		return "", fmt.Errorf("program %q is a path, not a name to find on PATH", name)
	}
	found, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("finding %s on PATH: %w", name, err)
	}
	if !filepath.IsAbs(found) {
		return "", fmt.Errorf("finding %s on PATH: %s is not an absolute path", name, found)
	}
	return found, nil
}

// Command is the command that runs program with args as its argument vector, bound to ctx. program
// is a bare name (resolved with Find) or an absolute path; the caller sets the directory,
// environment and streams on the result and runs it.
func Command(ctx context.Context, program string, args ...string) (*exec.Cmd, error) {
	resolved := program
	if !filepath.IsAbs(program) {
		found, err := Find(program)
		if err != nil {
			return nil, err
		}
		resolved = found
	}
	return exec.CommandContext(ctx, resolved, args...), nil
}
