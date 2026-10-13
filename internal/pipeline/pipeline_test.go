package pipeline_test

import (
	"reflect"
	"slices"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/pipeline"
)

var (
	fmtStep         = pipeline.Step{Name: "fmt", Phase: pipeline.PhaseFormat}
	suppressionStep = pipeline.Step{Name: "suppression", Phase: pipeline.PhaseDetectors, Rank: 1}
	golangciStep    = pipeline.Step{Name: "golangci", Phase: pipeline.PhaseDetectors, Rank: 5}
	analyzersStep   = pipeline.Step{Name: "analyzers", Phase: pipeline.PhaseDetectors, Rank: 6}
	testStep        = pipeline.Step{Name: "test", Phase: pipeline.PhaseRunners, Rank: 2}
	coverageStep    = pipeline.Step{Name: "coverage", Phase: pipeline.PhaseRunners, Rank: 3}
)

var selected = []pipeline.Step{fmtStep, suppressionStep, golangciStep, analyzersStep, testStep, coverageStep}

// fake records the steps it runs and fails the ones it is told to fail.
type fake struct {
	failing []pipeline.Step
	ran     []pipeline.Step
}

func (f *fake) Run(step pipeline.Step) pipeline.Outcome {
	f.ran = append(f.ran, step)
	if slices.Contains(f.failing, step) {
		return pipeline.Failed()
	}
	return pipeline.Passed()
}

func assertRan(t *testing.T, got *fake, want ...pipeline.Step) {
	t.Helper()
	if !reflect.DeepEqual(got.ran, want) {
		t.Errorf("ran %v, want %v", got.ran, want)
	}
}

func TestEverythingRunsInOrderWhenAllPass(t *testing.T) {
	exec := &fake{}
	report := pipeline.Execute(selected, exec, false)
	if !report.Passed {
		t.Error("report failed")
	}
	assertRan(t, exec, selected...)
}

func TestDetectorFailureRunsRemainingDetectorsButNoRunner(t *testing.T) {
	exec := &fake{failing: []pipeline.Step{suppressionStep}}
	report := pipeline.Execute(selected, exec, false)
	if report.Passed || !report.StoppedBeforeRunners {
		t.Errorf("passed=%v stopped=%v", report.Passed, report.StoppedBeforeRunners)
	}
	assertRan(t, exec, fmtStep, suppressionStep, golangciStep, analyzersStep)
}

func TestFailFastStopsAtTheFirstFailure(t *testing.T) {
	exec := &fake{failing: []pipeline.Step{suppressionStep}}
	report := pipeline.Execute(selected, exec, true)
	if report.Passed {
		t.Error("report passed")
	}
	assertRan(t, exec, fmtStep, suppressionStep)
}

func TestFormatFailureStillRunsDetectorsButNoRunner(t *testing.T) {
	exec := &fake{failing: []pipeline.Step{fmtStep}}
	report := pipeline.Execute(selected, exec, false)
	if !report.StoppedBeforeRunners {
		t.Error("runners were started")
	}
	assertRan(t, exec, fmtStep, suppressionStep, golangciStep, analyzersStep)
}

func TestRunnerFailureStopsLaterRunners(t *testing.T) {
	exec := &fake{failing: []pipeline.Step{testStep}}
	report := pipeline.Execute(selected, exec, false)
	if report.Passed || report.StoppedBeforeRunners {
		t.Errorf("passed=%v stopped=%v", report.Passed, report.StoppedBeforeRunners)
	}
	assertRan(t, exec, fmtStep, suppressionStep, golangciStep, analyzersStep, testStep)
}

type skipper struct{}

func (skipper) Run(pipeline.Step) pipeline.Outcome { return pipeline.Skipped("not configured") }

func TestSkippedStepsDoNotFailTheRun(t *testing.T) {
	report := pipeline.Execute(selected, skipper{}, false)
	if !report.Passed || len(report.Results) != len(selected) {
		t.Errorf("passed=%v results=%d", report.Passed, len(report.Results))
	}
	if report.Results[0].Outcome.Kind != pipeline.KindSkipped || report.Results[0].Outcome.Reason != "not configured" {
		t.Errorf("outcome = %+v", report.Results[0].Outcome)
	}
}

func TestInputOrderAndDuplicatesDoNotMatter(t *testing.T) {
	exec := &fake{}
	pipeline.Execute([]pipeline.Step{testStep, golangciStep, fmtStep, golangciStep, testStep}, exec, false)
	assertRan(t, exec, fmtStep, golangciStep, testStep)
}

func TestExecuteDoesNotReorderTheCallersSlice(t *testing.T) {
	in := []pipeline.Step{testStep, fmtStep}
	pipeline.Execute(in, &fake{}, false)
	if in[0] != testStep {
		t.Error("caller slice was sorted in place")
	}
}

func TestProjectStagesFollowTheBuiltInsOfTheirPhase(t *testing.T) {
	stageFmt := pipeline.Step{Name: "check", Phase: pipeline.PhaseFormat, Rank: pipeline.StageRank + 1}
	stageDet := pipeline.Step{Name: "lint", Phase: pipeline.PhaseDetectors, Rank: pipeline.StageRank}
	stageRun := pipeline.Step{Name: "suite", Phase: pipeline.PhaseRunners, Rank: pipeline.StageRank + 2}
	exec := &fake{}
	pipeline.Execute([]pipeline.Step{stageRun, testStep, stageDet, golangciStep, stageFmt, fmtStep}, exec, false)
	assertRan(t, exec, fmtStep, stageFmt, golangciStep, stageDet, testStep, stageRun)
}

func TestFailingDetectorStageStopsTheRunners(t *testing.T) {
	stageDet := pipeline.Step{Name: "lint", Phase: pipeline.PhaseDetectors, Rank: pipeline.StageRank}
	exec := &fake{failing: []pipeline.Step{stageDet}}
	report := pipeline.Execute([]pipeline.Step{stageDet, golangciStep, testStep}, exec, false)
	if !report.StoppedBeforeRunners {
		t.Error("runners were started")
	}
	assertRan(t, exec, golangciStep, stageDet)
}

func TestRunnerFailureIsNotReportedAsStoppedBeforeRunners(t *testing.T) {
	exec := &fake{failing: []pipeline.Step{testStep}}
	report := pipeline.Execute(selected, exec, true)
	if report.StoppedBeforeRunners {
		t.Error("a runner failed, so runners were started")
	}
}

func TestPhaseNames(t *testing.T) {
	for phase, want := range map[pipeline.Phase]string{
		pipeline.PhaseFormat: "format", pipeline.PhaseDetectors: "detectors", pipeline.PhaseRunners: "runners",
		pipeline.Phase(9): "unknown",
	} {
		if got := phase.String(); got != want {
			t.Errorf("phase %d = %q, want %q", phase, got, want)
		}
	}
}
