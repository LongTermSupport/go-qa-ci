// Package tools is the catalogue of go-qa tools in execution order.
package tools

import (
	"fmt"
	"strings"

	"github.com/LongTermSupport/go-qa-ci/internal/config"
	"github.com/LongTermSupport/go-qa-ci/internal/pipeline"
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

// Select returns the steps a run executes: the enabled tools when names is empty, otherwise the
// tools named (enabled or not), deduplicated. An unknown name is an error listing the known ones.
func Select(names []string, cfg config.Tools) ([]pipeline.Step, error) {
	catalogue := Catalogue()
	var steps []pipeline.Step
	if len(names) == 0 {
		for rank, tool := range catalogue {
			if tool.Enabled(cfg) {
				steps = append(steps, tool.Step(rank))
			}
		}
		return steps, nil
	}
	for _, name := range names {
		rank := indexOf(catalogue, name)
		if rank < 0 {
			return nil, fmt.Errorf("unknown tool %q; known tools: %s", name, strings.Join(Names(), ", "))
		}
		steps = append(steps, catalogue[rank].Step(rank))
	}
	return steps, nil
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
}

// Pending is the executor for tools whose engine is not built yet. It fails every step, saying
// so, so an unbuilt defence is never reported as passing.
type Pending struct{}

// Run fails the step.
func (Pending) Run(pipeline.Step) pipeline.Outcome {
	return pipeline.Outcome{Kind: pipeline.KindFailed, Reason: "not implemented yet"}
}
