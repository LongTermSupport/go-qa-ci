// Package tools is the catalogue of go-qa tools in execution order.
package tools

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/LongTermSupport/go-qa-ci/internal/changed"
	"github.com/LongTermSupport/go-qa-ci/internal/config"
	"github.com/LongTermSupport/go-qa-ci/internal/date"
	"github.com/LongTermSupport/go-qa-ci/internal/defences"
	"github.com/LongTermSupport/go-qa-ci/internal/pipeline"
	"github.com/LongTermSupport/go-qa-ci/internal/stage"
)

// Tool is one catalogue entry.
type Tool struct {
	// Name is the name used with `-t` and in the configuration.
	Name string
	// Phase is the phase the tool runs in.
	Phase pipeline.Phase
	// Enabled reports whether the configuration switches the tool on.
	Enabled func(config.Tools) bool
}

// Catalogue returns every tool in execution order.
func Catalogue() []Tool {
	return []Tool{
		{"fmt", pipeline.PhaseFormat, func(t config.Tools) bool { return t.Fmt }},
		{"record", pipeline.PhaseDetectors, func(t config.Tools) bool { return t.Record }},
		{"suppression", pipeline.PhaseDetectors, func(t config.Tools) bool { return t.Suppression }},
		{"summary", pipeline.PhaseDetectors, func(t config.Tools) bool { return t.Summary }},
		{"docs", pipeline.PhaseDetectors, func(t config.Tools) bool { return t.Docs }},
		{"ast-grep", pipeline.PhaseDetectors, func(t config.Tools) bool { return t.AstGrep }},
		{"golangci", pipeline.PhaseDetectors, func(t config.Tools) bool { return t.Golangci }},
		{"analyzers", pipeline.PhaseDetectors, func(t config.Tools) bool { return t.Analyzers }},
		{"deadcode", pipeline.PhaseDetectors, func(t config.Tools) bool { return t.Deadcode }},
		{"vuln", pipeline.PhaseRunners, func(t config.Tools) bool { return t.Vuln }},
		{"mod", pipeline.PhaseRunners, func(t config.Tools) bool { return t.Mod }},
		{"test", pipeline.PhaseRunners, func(t config.Tools) bool { return t.Test }},
		{"coverage", pipeline.PhaseRunners, func(t config.Tools) bool { return t.Coverage }},
		{"mutants", pipeline.PhaseRunners, func(t config.Tools) bool { return t.Mutants }},
	}
}

// FromName returns the tool a `-t` name selects.
func FromName(name string) (Tool, bool) {
	for _, tool := range Catalogue() {
		if tool.Name == name {
			return tool, true
		}
	}
	return Tool{}, false
}

// Select returns the steps a run executes: the enabled tools and every project stage when names is
// empty, otherwise the tools and stages named (enabled or not), deduplicated. An unknown name is an
// error listing the known ones.
func Select(names []string, cfg config.Config) ([]pipeline.Step, error) {
	catalogue := Catalogue()
	var steps []pipeline.Step
	if len(names) == 0 {
		for rank, tool := range catalogue {
			if tool.Enabled(cfg.Tools) {
				steps = append(steps, tool.Step(rank))
			}
		}
		for i := range cfg.Stages {
			step, err := stageStep(cfg, i)
			if err != nil {
				return nil, err
			}
			steps = append(steps, step)
		}
		return steps, nil
	}
	for _, name := range names {
		if rank := indexOf(catalogue, name); rank >= 0 {
			steps = append(steps, catalogue[rank].Step(rank))
			continue
		}
		i := slices.IndexFunc(cfg.Stages, func(s stage.Stage) bool { return s.Name == name })
		if i < 0 {
			return nil, fmt.Errorf("unknown tool %q; known tools and stages: %s", name, strings.Join(slices.Concat(Names(), StageNames(cfg)), ", "))
		}
		step, err := stageStep(cfg, i)
		if err != nil {
			return nil, err
		}
		steps = append(steps, step)
	}
	return steps, nil
}

// stageStep is the step of the project stage declared at position i; its rank follows every
// built-in tool of its phase, in declared order.
func stageStep(cfg config.Config, i int) (pipeline.Step, error) {
	s := cfg.Stages[i]
	phase, err := s.PhaseOf()
	if err != nil {
		return pipeline.Step{}, fmt.Errorf("selecting a stage: %w", err)
	}
	return pipeline.Step{Name: s.Name, Phase: phase, Rank: pipeline.StageRank + i}, nil
}

// StageNames lists the names of the project stages in declared order.
func StageNames(cfg config.Config) []string {
	names := make([]string, len(cfg.Stages))
	for i, s := range cfg.Stages {
		names[i] = s.Name
	}
	return names
}

// CheckStages fails when a project stage takes the name of a built-in tool, which `-t` could then
// no longer tell apart.
func CheckStages(cfg config.Config) error {
	var problems []string
	for _, s := range cfg.Stages {
		if _, builtin := FromName(s.Name); builtin {
			problems = append(problems, fmt.Sprintf("stage `%s` has the name of a built-in tool", s.Name))
		}
	}
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

// Names lists every tool name in execution order.
func Names() []string {
	catalogue := Catalogue()
	names := make([]string, len(catalogue))
	for i, tool := range catalogue {
		names[i] = tool.Name
	}
	return names
}

// Step is the pipeline step for the tool at catalogue position rank.
func (t Tool) Step(rank int) pipeline.Step {
	return pipeline.Step{Name: t.Name, Phase: t.Phase, Rank: rank}
}

func indexOf(catalogue []Tool, name string) int {
	for i, tool := range catalogue {
		if tool.Name == name {
			return i
		}
	}
	return -1
}

// Context is what a run tells its executor about the project and the invocation.
type Context struct {
	// Root is the project root.
	Root string
	// Config is the effective configuration.
	Config config.Config
	// Scope is the paths the run is limited to, relative to Root; empty means the whole project.
	Scope []string
	// CI reports CI mode: autofix, then fail if the working tree changed.
	CI bool
	// Fix reports whether the format phase may modify files.
	Fix bool
	// Today is the date deadlines are measured against.
	Today date.Date
	// Diff is the plan of a diff-mode run (`run --changed`); nil for a full run.
	Diff *changed.Plan
}

// Pending is the executor for a step no engine is built for. It fails every step, saying
// so, so an unbuilt defence is never reported as passing.
type Pending struct{}

// Run fails the step.
func (Pending) Run(pipeline.Step) pipeline.Outcome {
	return pipeline.Outcome{Kind: pipeline.KindFailed, Reason: "not implemented yet"}
}

// Steps lists every step of the gate in execution order, as the defence listing reports them: per
// phase, the built-in tools first and then the project's own stages in declared order.
func Steps(cfg config.Config) []defences.Step {
	var steps []defences.Step
	for _, phase := range []pipeline.Phase{pipeline.PhaseFormat, pipeline.PhaseDetectors, pipeline.PhaseRunners} {
		for _, tool := range Catalogue() {
			if tool.Phase == phase {
				steps = append(steps, defences.Step{Name: tool.Name, Phase: phase.String(), Enabled: tool.Enabled(cfg.Tools)})
			}
		}
		for _, s := range cfg.Stages {
			if s.Phase == phase.String() {
				steps = append(steps, defences.Step{Name: s.Name, Phase: s.Phase, Project: true, Enabled: true})
			}
		}
	}
	return steps
}
