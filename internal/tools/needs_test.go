package tools_test

import (
	"reflect"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/config"
	"github.com/LongTermSupport/go-qa-ci/internal/pipeline"
	"github.com/LongTermSupport/go-qa-ci/internal/tools"
)

func stepsNamed(names ...string) []pipeline.Step {
	steps := make([]pipeline.Step, 0, len(names))
	for _, n := range names {
		steps = append(steps, pipeline.Step{Name: n})
	}
	return steps
}

func TestNeedsNamesTheExternalToolOfEachStep(t *testing.T) {
	cfg := config.Default()
	got := tools.Needs(stepsNamed("fmt", "ast-grep", "golangci", "deadcode", "vuln", "mutants", "test", "record"), cfg)
	want := []string{"ast-grep", "deadcode", "golangci-lint", "govulncheck", "gremlins"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestNeedsAsksForGolangciOnceWhateverUsesIt(t *testing.T) {
	got := tools.Needs(stepsNamed("golangci", "fmt", "summary"), config.Default())
	if !reflect.DeepEqual(got, []string{"golangci-lint"}) {
		t.Errorf("got %v", got)
	}
}

func TestNeedsSummaryNeedsGolangciOnlyWhenTheLintersAreListed(t *testing.T) {
	cfg := config.Default()
	cfg.Tools.Golangci = false
	if got := tools.Needs(stepsNamed("summary"), cfg); len(got) != 0 {
		t.Errorf("got %v", got)
	}
}

func TestNeedsNothingForStepsWithoutAnExternalTool(t *testing.T) {
	if got := tools.Needs(stepsNamed("record", "suppression", "docs", "analyzers", "mod", "test", "coverage"), config.Default()); len(got) != 0 {
		t.Errorf("got %v", got)
	}
}
