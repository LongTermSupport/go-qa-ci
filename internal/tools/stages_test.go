package tools_test

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/config"
	"github.com/LongTermSupport/go-qa-ci/internal/faketool"
	"github.com/LongTermSupport/go-qa-ci/internal/pipeline"
	"github.com/LongTermSupport/go-qa-ci/internal/stage"
	"github.com/LongTermSupport/go-qa-ci/internal/tools"
)

// The glob is assembled from parts so that no source line holds a comment opener.
const anyShell = "**/" + "*.sh"

// shellStage is a ShellCheck-style stage over every shell script.
func shellStage() stage.Stage {
	return stage.Stage{
		Name: "shellcheck", Phase: "detectors", Says: "ShellCheck over every shell script", Fix: "Quote expansions",
		Command: []string{"shellcheck", stage.Files}, Files: []string{anyShell},
	}
}

// webStage reports findings as JSON from the web directory.
func webStage() stage.Stage {
	return stage.Stage{
		Name: "weblint", Phase: "detectors", Says: "Lint the web code", Fix: "Follow the finding's page",
		Command: []string{"web-lint"}, Dir: "web", Output: stage.OutputFindings, Rules: []string{"web-rules"},
	}
}

func runStage(t *testing.T, root string, stages []stage.Stage, scope ...string) (pipeline.Outcome, string) {
	t.Helper()
	cfg := config.Default()
	cfg.Stages = stages
	steps, err := tools.Select([]string{stages[0].Name}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	exec := tools.Executor{Ctx: tools.Context{Root: root, Config: cfg, Scope: scope, Today: fixedToday(t)}, Stderr: &stderr}
	return exec.Run(steps[0]), stderr.String()
}

func TestAStageOverShellScriptsIsSkippedWhenNoFilesMatch(t *testing.T) {
	log := faketool.Install(t, "shellcheck", faketool.Spec{})
	root := fixture(t, map[string]string{"main.go": cleanSource})
	outcome, stderr := runStage(t, root, []stage.Stage{shellStage()})
	if outcome.Kind != pipeline.KindSkipped || !strings.Contains(outcome.Reason, "no files") || stderr != "" {
		t.Errorf("got %+v\n%s", outcome, stderr)
	}
	if calls := faketool.Calls(t, log); len(calls) != 0 {
		t.Errorf("the tool ran with nothing to check: %q", calls)
	}
}

func TestAStatusStagePassesOrFailsOnTheExitCodeAndShowsItsOutput(t *testing.T) {
	log := faketool.Install(t, "shellcheck", faketool.Spec{Stdout: "all quiet\n"})
	root := fixture(t, map[string]string{"scripts/a.sh": "echo a\n", "scripts/b c.sh": "echo b\n", "docs/readme.md": "x\n"})
	outcome, stderr := runStage(t, root, []stage.Stage{shellStage()})
	if outcome.Kind != pipeline.KindPassed || !strings.Contains(stderr, "all quiet") {
		t.Errorf("got %+v\n%s", outcome, stderr)
	}
	if calls := faketool.Calls(t, log); len(calls) != 1 || !strings.HasSuffix(calls[0], "\tscripts/a.sh scripts/b c.sh") {
		t.Errorf("calls = %q", calls)
	}

	faketool.Install(t, "shellcheck", faketool.Spec{Stdout: "SC2086 here\n", Code: 1})
	outcome, stderr = runStage(t, root, []stage.Stage{shellStage()})
	if outcome.Kind != pipeline.KindFailed || !strings.Contains(stderr, "SC2086 here") || !strings.Contains(stderr, "error[shellcheck]: ") || !strings.Contains(stderr, "exited with code 1") {
		t.Errorf("got %+v\n%s", outcome, stderr)
	}
}

func TestAScopedRunUsesTheFilesCommandOnlyOnTheScopedFiles(t *testing.T) {
	log := faketool.Install(t, "fmtcheck", faketool.Spec{})
	root := fixture(t, map[string]string{"scripts/a.sh": "echo a\n", "other/b.sh": "echo b\n"})
	s := shellStage()
	s.Name = "fmtcheck"
	s.Command = []string{"fmtcheck", "--all"}
	s.FilesCommand = []string{"fmtcheck", "--files", stage.Files}
	if outcome, stderr := runStage(t, root, []stage.Stage{s}, "scripts"); outcome.Kind != pipeline.KindPassed {
		t.Fatalf("got %+v\n%s", outcome, stderr)
	}
	if outcome, stderr := runStage(t, root, []stage.Stage{s}); outcome.Kind != pipeline.KindPassed {
		t.Fatalf("got %+v\n%s", outcome, stderr)
	}
	calls := faketool.Calls(t, log)
	if len(calls) != 2 || !strings.HasSuffix(calls[0], "\t--files scripts/a.sh") || !strings.HasSuffix(calls[1], "\t--all") {
		t.Errorf("calls = %q", calls)
	}
}

func TestAStageRunsInItsDirectoryWithFilesRelativeToIt(t *testing.T) {
	log := faketool.Install(t, "web-test", faketool.Spec{})
	root := fixture(t, map[string]string{"web/src/a.ts": "x\n", "web/package.json": "{}\n"})
	s := stage.Stage{
		Name: "web-test", Phase: "runners", Says: "s", Fix: "f", Dir: "web",
		Command: []string{"web-test", stage.Files}, Files: []string{"web/" + "**/" + "*.ts"},
	}
	if outcome, stderr := runStage(t, root, []stage.Stage{s}); outcome.Kind != pipeline.KindPassed {
		t.Fatalf("got %+v\n%s", outcome, stderr)
	}
	want, err := filepath.EvalSymlinks(filepath.Join(root, "web"))
	if err != nil {
		t.Fatal(err)
	}
	if calls := faketool.Calls(t, log); len(calls) != 1 || calls[0] != want+"\tsrc/a.ts" {
		t.Errorf("calls = %q, want %s and src/a.ts", calls, want)
	}
}

func TestAMissingProgramFailsTheStage(t *testing.T) {
	root := fixture(t, map[string]string{"scripts/a.sh": "echo a\n"})
	s := shellStage()
	s.Command = []string{"no-such-program-example", stage.Files}
	outcome, stderr := runStage(t, root, []stage.Stage{s})
	if outcome.Kind != pipeline.KindFailed || !strings.Contains(stderr, "no-such-program-example") {
		t.Errorf("got %+v\n%s", outcome, stderr)
	}
}

const webFindings = `[{"file":"src/a.ts","line":4,"rule":"web-rule","message":"avoid this","note":"use that"},` +
	`{"file":"src/b.ts","line":9,"rule":"web-rule","message":"avoid this too"}]`

const webRules = `[{"id":"web-rule","description":"what it finds"}]`

const licenseA = "[[exception]]\nrule = \"web-rule\"\npath = \"web/src/a.ts\"\ncovers = \"findings\"\njustification = \"the fixture does this on purpose\"\n"

func TestFindingsAreReportedCompilerStyleWithPathsFromTheRoot(t *testing.T) {
	faketool.Install(t, "web-lint", faketool.Spec{Stdout: webFindings, Code: 1})
	root := fixture(t, map[string]string{"web/src/a.ts": "x\n"})
	outcome, stderr := runStage(t, root, []stage.Stage{webStage()}, "web")
	if outcome.Kind != pipeline.KindFailed {
		t.Fatalf("got %+v\n%s", outcome, stderr)
	}
	for _, want := range []string{"error[web-rule]: avoid this (use that)\n  --> web/src/a.ts:4", "error[web-rule]: avoid this too\n  --> web/src/b.ts:9"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("no %q in:\n%s", want, stderr)
		}
	}
}

func TestTheRecordIsAppliedToAStagesFindings(t *testing.T) {
	faketool.Install(t, "web-lint", faketool.Spec{Stdout: webFindings, Code: 1})
	root := fixture(t, map[string]string{"web/src/a.ts": "x\n"})
	writeRecord(t, root, licenseA)
	outcome, stderr := runStage(t, root, []stage.Stage{webStage()}, "web")
	if outcome.Kind != pipeline.KindFailed || strings.Contains(stderr, "src/a.ts") || !strings.Contains(stderr, "web/src/b.ts:9") {
		t.Errorf("only the unlicensed finding must remain: %+v\n%s", outcome, stderr)
	}
}

func TestAStagePassesWhenTheRecordLicensesEveryFinding(t *testing.T) {
	faketool.Install(t, "web-lint", faketool.Spec{Stdout: `[{"file":"src/a.ts","line":4,"rule":"web-rule","message":"m"}]`, Code: 1})
	root := fixture(t, map[string]string{"web/src/a.ts": "x\n"})
	writeRecord(t, root, licenseA)
	outcome, stderr := runStage(t, root, []stage.Stage{webStage()}, "web")
	if outcome.Kind != pipeline.KindPassed || stderr != "" {
		t.Errorf("got %+v\n%s", outcome, stderr)
	}
}

func TestFindingsOutsideTheScopeAreIgnored(t *testing.T) {
	faketool.Install(t, "web-lint", faketool.Spec{Stdout: webFindings, Code: 1})
	root := fixture(t, map[string]string{"web/src/a.ts": "x\n"})
	outcome, stderr := runStage(t, root, []stage.Stage{webStage()}, "web/src/a.ts")
	if outcome.Kind != pipeline.KindFailed || strings.Contains(stderr, "b.ts") || !strings.Contains(stderr, "a.ts:4") {
		t.Errorf("got %+v\n%s", outcome, stderr)
	}
}

func TestAFindingsStageThatDoesNotCompleteFails(t *testing.T) {
	root := fixture(t, map[string]string{"web/src/a.ts": "x\n"})
	for name, spec := range map[string]faketool.Spec{
		"exit code 2":                 {Stdout: "[]", Stderr: "boom", Code: 2},
		"not json":                    {Stdout: "oops"},
		"exit 1 with no findings":     {Stdout: "[]", Code: 1},
		"unknown field in the report": {Stdout: `[{"file":"a","line":1,"rule":"r","message":"m","severity":"x"}]`, Code: 1},
	} {
		faketool.Install(t, "web-lint", spec)
		outcome, stderr := runStage(t, root, []stage.Stage{webStage()})
		if outcome.Kind != pipeline.KindFailed || !strings.Contains(stderr, "error[weblint]: ") {
			t.Errorf("%s: got %+v\n%s", name, outcome, stderr)
		}
	}
}

func TestAFindingsStageThatFindsNothingPassesQuietly(t *testing.T) {
	faketool.Install(t, "web-lint", faketool.Spec{Stdout: "[]"})
	faketool.Install(t, "web-rules", faketool.Spec{Stdout: webRules})
	root := fixture(t, map[string]string{"web/src/a.ts": "x\n"})
	if outcome, stderr := runStage(t, root, []stage.Stage{webStage()}); outcome.Kind != pipeline.KindPassed || stderr != "" {
		t.Errorf("got %+v\n%s", outcome, stderr)
	}
}

func TestAStageRulesExceptionThatLicensesNothingIsStaleOnAWholeRunOnly(t *testing.T) {
	faketool.Install(t, "web-lint", faketool.Spec{Stdout: "[]"})
	faketool.Install(t, "web-rules", faketool.Spec{Stdout: webRules})
	root := fixture(t, map[string]string{"web/src/a.ts": "x\n"})
	writeRecord(t, root, licenseA)
	outcome, stderr := runStage(t, root, []stage.Stage{webStage()})
	if outcome.Kind != pipeline.KindFailed || !strings.Contains(stderr, "error[gqaci.record.stale]") {
		t.Errorf("a whole run must call the unused exception stale: %+v\n%s", outcome, stderr)
	}
	scoped, scopedStderr := runStage(t, root, []stage.Stage{webStage()}, "web")
	if scoped.Kind != pipeline.KindPassed || scopedStderr != "" {
		t.Errorf("a scoped run cannot judge staleness: %+v\n%s", scoped, scopedStderr)
	}
}

func TestSelectPutsStagesAfterTheBuiltinsOfTheirPhaseInDeclaredOrder(t *testing.T) {
	cfg := config.Default()
	runner := stage.Stage{Name: "e2e", Phase: "runners", Says: "s", Fix: "f", Command: []string{"e2e"}}
	format := stage.Stage{Name: "prettier", Phase: "format", Says: "s", Fix: "f", Command: []string{"prettier"}}
	cfg.Stages = []stage.Stage{runner, webStage(), format}
	steps, err := tools.Select(nil, cfg)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, step := range steps {
		names = append(names, step.Name)
	}
	report := pipeline.Execute(steps, recordingExecutor{}, false)
	var ran []string
	for _, result := range report.Results {
		ran = append(ran, result.Step.Name)
	}
	order := strings.Join(ran, " ")
	if !strings.Contains(order, "fmt prettier record") || !strings.Contains(order, "deadcode weblint vuln") || !strings.HasSuffix(order, "mutants e2e") {
		t.Errorf("order = %s (selected %v)", order, names)
	}
}

type recordingExecutor struct{}

func (recordingExecutor) Run(pipeline.Step) pipeline.Outcome { return pipeline.Passed() }

func TestAStageCanBeSelectedByNameAndAnUnknownNameListsStagesToo(t *testing.T) {
	cfg := config.Default()
	cfg.Stages = []stage.Stage{webStage()}
	steps, err := tools.Select([]string{"weblint"}, cfg)
	if err != nil || len(steps) != 2 || steps[0].Name != "weblint" || steps[1].Name != "record" || steps[0].Phase != pipeline.PhaseDetectors || steps[0].Rank < pipeline.StageRank {
		t.Errorf("steps = %+v, %v", steps, err)
	}
	_, err = tools.Select([]string{"nonesuch"}, cfg)
	if err == nil || !strings.Contains(err.Error(), "weblint") || !strings.Contains(err.Error(), "golangci") {
		t.Errorf("err = %v", err)
	}
}

func TestAStageMayNotTakeTheNameOfABuiltInTool(t *testing.T) {
	cfg := config.Default()
	clash := webStage()
	clash.Name = "golangci"
	cfg.Stages = []stage.Stage{clash}
	if err := tools.CheckStages(cfg); err == nil || !strings.Contains(err.Error(), "built-in tool") {
		t.Errorf("err = %v", err)
	}
	cfg.Stages = []stage.Stage{webStage()}
	if err := tools.CheckStages(cfg); err != nil {
		t.Error(err)
	}
}
