// Package pipeline orders and executes the steps of a run.
//
// Steps run in phase order: format, then detectors, then runners. Within a phase the built-in
// tools come first in catalogue order, then the project's own stages in declared order. Every
// selected step of the format and detector phases runs (unless fail-fast), so one invocation
// reports all static findings; a failure in either phase stops the run before the first runner
// (TOOLING-SPEC 4.5). Runners stop at the first failure.
package pipeline

import (
	"cmp"
	"fmt"
	"slices"
)

// Phase is a pipeline phase, in execution order.
type Phase int

// The phases, in execution order.
const (
	// PhaseFormat formats the code (modifies locally, checks in CI).
	PhaseFormat Phase = iota
	// PhaseDetectors holds the static detectors: they read code and never execute it.
	PhaseDetectors
	// PhaseRunners holds the steps that execute code or audit dependencies.
	PhaseRunners
)

// String is the phase name.
func (p Phase) String() string {
	switch p {
	case PhaseFormat:
		return "format"
	case PhaseDetectors:
		return "detectors"
	case PhaseRunners:
		return "runners"
	default:
		return "unknown"
	}
}

// ParsePhase is the phase a configuration names.
func ParsePhase(name string) (Phase, error) {
	for _, phase := range []Phase{PhaseFormat, PhaseDetectors, PhaseRunners} {
		if phase.String() == name {
			return phase, nil
		}
	}
	return 0, fmt.Errorf("unknown phase %q (want format, detectors or runners)", name)
}

// StageRank is the first Rank of a project stage; built-in tools rank below it, so a phase runs
// its built-ins before its stages.
const StageRank = 1000

// Step is one unit of a run: a built-in tool or a project stage.
type Step struct {
	// Name is the name shown in the summary and accepted by `-t`.
	Name string
	// Phase is the phase the step belongs to.
	Phase Phase
	// Rank orders the steps of one phase: catalogue position for built-ins, StageRank plus the
	// declared position for project stages.
	Rank int
}

func compare(a, b Step) int {
	return cmp.Or(cmp.Compare(a.Phase, b.Phase), cmp.Compare(a.Rank, b.Rank), cmp.Compare(a.Name, b.Name))
}

// Kind classifies an Outcome.
type Kind int

// The outcome kinds.
const (
	// KindPassed means the step passed.
	KindPassed Kind = iota
	// KindFailed means the step failed; its own output explains why.
	KindFailed
	// KindSkipped means the step does not apply to this project (Reason says why).
	KindSkipped
)

// Outcome is the result of one step.
type Outcome struct {
	Kind   Kind
	Reason string
}

// Passed is the outcome of a step that passed.
func Passed() Outcome { return Outcome{Kind: KindPassed} }

// Failed is the outcome of a step that failed.
func Failed() Outcome { return Outcome{Kind: KindFailed} }

// Skipped is the outcome of a step that does not apply, with the reason.
func Skipped(reason string) Outcome { return Outcome{Kind: KindSkipped, Reason: reason} }

// Executor runs one step.
type Executor interface {
	Run(step Step) Outcome
}

// Result pairs a step with its outcome.
type Result struct {
	Step    Step
	Outcome Outcome
}

// Report is the outcome of a pipeline run.
type Report struct {
	// Results holds the steps that ran, in order.
	Results []Result
	// StoppedBeforeRunners reports that a format or detector failure kept the runners from starting.
	StoppedBeforeRunners bool
	// Passed reports that every step that ran passed or was skipped.
	Passed bool
}

// Execute runs the selected steps in order with the stop rules described in the package docs.
func Execute(selected []Step, executor Executor, failFast bool) Report {
	steps := slices.Clone(selected)
	slices.SortFunc(steps, compare)
	steps = slices.CompactFunc(steps, func(a, b Step) bool { return compare(a, b) == 0 })
	report := Report{Passed: true}
	for _, step := range steps {
		if !report.Passed && (failFast || step.Phase == PhaseRunners) {
			report.StoppedBeforeRunners = !slices.ContainsFunc(report.Results, func(r Result) bool {
				return r.Step.Phase == PhaseRunners
			})
			break
		}
		outcome := executor.Run(step)
		if outcome.Kind == KindFailed {
			report.Passed = false
		}
		report.Results = append(report.Results, Result{Step: step, Outcome: outcome})
	}
	return report
}
