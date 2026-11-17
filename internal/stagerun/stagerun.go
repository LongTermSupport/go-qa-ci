// Package stagerun launches the commands of project stages: the one that prints the rules a stage
// reports, and the stage's own command. A program that is not installed is an error, never a
// skipped check.
package stagerun

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/LongTermSupport/go-qa-ci/internal/runners"
	"github.com/LongTermSupport/go-qa-ci/internal/stage"
)

// Dir is the directory a stage's commands run in: the stage directory inside the root, or the root.
func Dir(root string, s stage.Stage) string {
	return filepath.Join(root, filepath.FromSlash(s.Dir))
}

// Capture runs the command line argv for the stage and returns what it printed.
func Capture(ctx context.Context, root string, s stage.Stage, argv []string) (runners.Output, error) {
	if len(argv) == 0 {
		return runners.Output{}, fmt.Errorf("stage `%s` has an empty command", s.Name)
	}
	dir := Dir(root, s)
	program, err := programIn(dir, argv[0])
	if err != nil {
		return runners.Output{}, fmt.Errorf("stage `%s`: %w", s.Name, err)
	}
	out, err := runners.Capture(ctx, dir, program, argv[1:]...)
	if err != nil {
		return runners.Output{}, fmt.Errorf("stage `%s`: %w", s.Name, err)
	}
	return out, nil
}

// programIn is the program a stage command names, as launch takes it: a bare name stays a name to
// find on PATH, and a path (`./scripts/lint.sh`) is resolved against the stage directory, the
// directory it is written relative to, into an absolute path.
func programIn(dir, program string) (string, error) {
	if filepath.IsAbs(program) || !strings.ContainsRune(program, '/') && !strings.ContainsRune(program, filepath.Separator) {
		return program, nil
	}
	abs, err := filepath.Abs(filepath.Join(dir, filepath.FromSlash(program)))
	if err != nil {
		return "", fmt.Errorf("resolving %s: %w", program, err)
	}
	return abs, nil
}

// LoadRules are the rules the stage reports, from its rules command; a stage without one reports
// none. The command failing, or printing anything but a JSON list of rules, is an error.
func LoadRules(ctx context.Context, root string, s stage.Stage) ([]stage.Rule, error) {
	if s.Rules == nil {
		return nil, nil
	}
	line := strings.Join(s.Rules, " ")
	out, err := Capture(ctx, root, s, s.Rules)
	if err != nil {
		return nil, err
	}
	if out.Code != 0 {
		return nil, fmt.Errorf("stage `%s`: `%s` exited %d: %s", s.Name, line, out.Code, strings.TrimSpace(out.Stderr))
	}
	rules, err := stage.ParseRules(out.Stdout)
	if err != nil {
		return nil, fmt.Errorf("stage `%s`: `%s` printed %w", s.Name, line, err)
	}
	return rules, nil
}
