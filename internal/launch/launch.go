// Package launch is the one place go-qa's code starts another program, so the hazard gosec G204
// guards against (command injection through a shell or an attacker-chosen program) is answered
// once, here, and every other file is held to the rule.
//
// The contract of every launch:
//
//   - an argument vector only: each argument reaches the program as one value, and no shell parses
//     it. A shell handed a command string (sh -c, bash -lc, behind env and the like) is refused;
//     a shell running a script file is an ordinary program;
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
	"path"
	"path/filepath"
	"slices"
	"strings"
)

// shells are the programs that take a command string with -c.
var shells = []string{"sh", "bash", "zsh", "dash", "ash", "ksh", "csh", "tcsh", "fish"}

// wrappers run the program named after their own options and assignments.
var wrappers = []string{"env", "sudo", "doas", "nohup", "nice", "timeout", "time", "stdbuf", "setsid", "xargs", "busybox"}

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
	if shell, option := shellWithCommandString(append([]string{program}, args...)); shell != "" {
		return nil, fmt.Errorf("refusing to run the shell %s with %s: a command string is parsed again by the shell; pass the program and its arguments separately", shell, option)
	}
	return exec.CommandContext(ctx, resolved, args...), nil
}

// shellWithCommandString finds a shell at the start of argv, behind any wrapper and its options or
// assignments, and the option among the shell's leading options that hands it a command string.
func shellWithCommandString(argv []string) (shell, option string) {
	for i, arg := range argv {
		base := path.Base(filepath.ToSlash(arg))
		switch {
		case slices.Contains(shells, base):
			if option = commandOption(argv[i+1:]); option != "" {
				return base, option
			}
			return "", ""
		case i == 0 && slices.Contains(wrappers, base):
		case i > 0 && (strings.HasPrefix(arg, "-") || strings.Contains(arg, "=")):
		default:
			return "", ""
		}
	}
	return "", ""
}

// commandOption is the -c style option (-c, or a cluster of short options ending in c such as -lc)
// among the leading options of args.
func commandOption(args []string) string {
	for _, arg := range args {
		if !strings.HasPrefix(arg, "-") || arg == "-" || arg == "--" {
			return ""
		}
		if !strings.HasPrefix(arg, "--") && strings.HasSuffix(arg, "c") && isLetters(arg[1:]) {
			return arg
		}
	}
	return ""
}

func isLetters(s string) bool {
	return s != "" && strings.IndexFunc(s, func(r rune) bool { return (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') }) < 0
}
