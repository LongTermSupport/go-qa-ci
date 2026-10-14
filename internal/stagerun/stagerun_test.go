package stagerun_test

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/faketool"
	"github.com/LongTermSupport/go-qa-ci/internal/stage"
	"github.com/LongTermSupport/go-qa-ci/internal/stagerun"
)

func TestMain(m *testing.M) {
	faketool.Main()
	os.Exit(m.Run())
}

func stageWithRules(command ...string) stage.Stage {
	return stage.Stage{Name: "web", Phase: "detectors", Says: "s", Fix: "f", Command: []string{"web-lint"}, Rules: command, Dir: "web"}
}

func project(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "web"), 0o750); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestLoadRulesRunsTheRulesCommandInTheStageDirectory(t *testing.T) {
	log := faketool.Install(t, "web-rules", faketool.Spec{Stdout: `[{"id":"web-rule","description":"finds things"}]`})
	root := project(t)
	rules, err := stagerun.LoadRules(context.Background(), root, stageWithRules("web-rules", "--list"))
	if err != nil || !reflect.DeepEqual(rules, []stage.Rule{{ID: "web-rule", Description: "finds things"}}) {
		t.Fatalf("rules = %+v, %v", rules, err)
	}
	calls := faketool.Calls(t, log)
	resolved, evalErr := filepath.EvalSymlinks(filepath.Join(root, "web"))
	if evalErr != nil || len(calls) != 1 || calls[0] != resolved+"\t--list" {
		t.Errorf("calls = %q, want one in %s", calls, resolved)
	}
}

func TestAStageWithoutARulesCommandReportsNoRules(t *testing.T) {
	rules, err := stagerun.LoadRules(context.Background(), project(t), stageWithRules())
	if err != nil || len(rules) != 0 {
		t.Errorf("rules = %v, %v", rules, err)
	}
}

func TestARulesCommandThatFailsOrPrintsNoRuleListIsAnError(t *testing.T) {
	faketool.Install(t, "bad-exit", faketool.Spec{Stderr: "cannot list", Code: 3})
	faketool.Install(t, "bad-json", faketool.Spec{Stdout: "not json"})
	for command, want := range map[string]string{"bad-exit": "cannot list", "bad-json": "not a JSON list of rules"} {
		_, err := stagerun.LoadRules(context.Background(), project(t), stageWithRules(command))
		if err == nil || !strings.Contains(err.Error(), want) || !strings.Contains(err.Error(), command) {
			t.Errorf("%s: %v", command, err)
		}
	}
}

func TestAMissingRulesProgramIsAnErrorNotAnEmptyList(t *testing.T) {
	_, err := stagerun.LoadRules(context.Background(), project(t), stageWithRules("no-such-program-example"))
	if err == nil {
		t.Error("a missing program passed")
	}
}
