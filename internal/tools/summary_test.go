package tools_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/config"
	"github.com/LongTermSupport/go-qa-ci/internal/pipeline"
	"github.com/LongTermSupport/go-qa-ci/internal/stage"
	"github.com/LongTermSupport/go-qa-ci/internal/tools"
)

func TestStepsAreInExecutionOrderWithProjectStagesAfterTheBuiltInsOfTheirPhase(t *testing.T) {
	cfg := config.Default()
	cfg.Tools.Mutants = false
	cfg.Stages = []stage.Stage{
		{Name: "e2e", Phase: "runners"},
		{Name: "examples", Phase: "detectors"},
	}
	var got []string
	for _, s := range tools.Steps(cfg) {
		got = append(got, s.Phase+":"+s.Name)
	}
	joined := strings.Join(got, " ")
	for _, want := range []string{
		"format:fmt detectors:record",
		"detectors:deadcode detectors:examples runners:vuln",
		"runners:coverage runners:mutants runners:e2e",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("order lacks %q:\n%s", want, joined)
		}
	}
	for _, s := range tools.Steps(cfg) {
		if s.Name == "mutants" && s.Enabled || s.Name == "e2e" && (!s.Enabled || !s.Project) || s.Name == "fmt" && (!s.Enabled || s.Project) {
			t.Errorf("wrong flags: %+v", s)
		}
	}
}

func summaryExecutor(root string, cfg config.Config, stderr *bytes.Buffer) tools.Executor {
	return tools.Executor{Ctx: tools.Context{Root: root, Config: cfg}, Stderr: stderr}
}

func TestSummaryToolIsOffWithoutAFileAndSaysSo(t *testing.T) {
	var stderr bytes.Buffer
	outcome := summaryExecutor(t.TempDir(), config.Default(), &stderr).Run(pipeline.Step{Name: "summary"})
	if outcome.Kind != pipeline.KindPassed || !strings.Contains(stderr.String(), "summary.file") {
		t.Errorf("outcome %+v, stderr %q", outcome, stderr.String())
	}
}

func TestSummaryToolFailsOnAMissingRegion(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/m\n\ngo 1.26\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Summary.File = "AGENTS.md"
	cfg.Tools.Golangci = false
	cfg.Tools.Analyzers = false
	var stderr bytes.Buffer
	outcome := summaryExecutor(root, cfg, &stderr).Run(pipeline.Step{Name: "summary"})
	if outcome.Kind != pipeline.KindFailed || !strings.Contains(stderr.String(), "gqaci.summary.stale") {
		t.Errorf("outcome %+v, stderr %q", outcome, stderr.String())
	}
}
