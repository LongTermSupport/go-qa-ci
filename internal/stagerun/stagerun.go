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
	out, err := runners.Capture(ctx, Dir(root, s), argv[0], argv[1:]...)
	if err != nil {
		return runners.Output{}, fmt.Errorf("stage `%s`: %w", s.Name, err)
	}
	return out, nil
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
