// Package tools is the catalogue of go-qa tools in execution order.
package tools

import "github.com/LongTermSupport/go-qa-ci/internal/config"

// Phase is a pipeline phase: format, then detectors, then runners.
type Phase int

// The phases, in execution order.
const (
	PhaseFormat Phase = iota
	PhaseDetectors
	PhaseRunners
)

// String is the phase name shown by `go-qa tools`.
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

// Tool is one catalogue entry.
type Tool struct {
	// Name is the name used with `-t` and in the configuration.
	Name string
	// Phase is the phase the tool runs in.
	Phase Phase
	// Enabled reports whether the configuration switches the tool on.
	Enabled func(config.Tools) bool
}

// Catalogue returns every tool in execution order.
func Catalogue() []Tool {
	return []Tool{
		{"fmt", PhaseFormat, func(t config.Tools) bool { return t.Fmt }},
		{"record", PhaseDetectors, func(t config.Tools) bool { return t.Record }},
		{"suppression", PhaseDetectors, func(t config.Tools) bool { return t.Suppression }},
		{"summary", PhaseDetectors, func(t config.Tools) bool { return t.Summary }},
		{"docs", PhaseDetectors, func(t config.Tools) bool { return t.Docs }},
		{"ast-grep", PhaseDetectors, func(t config.Tools) bool { return t.AstGrep }},
		{"golangci", PhaseDetectors, func(t config.Tools) bool { return t.Golangci }},
		{"analyzers", PhaseDetectors, func(t config.Tools) bool { return t.Analyzers }},
		{"deadcode", PhaseDetectors, func(t config.Tools) bool { return t.Deadcode }},
		{"vuln", PhaseRunners, func(t config.Tools) bool { return t.Vuln }},
		{"mod", PhaseRunners, func(t config.Tools) bool { return t.Mod }},
		{"test", PhaseRunners, func(t config.Tools) bool { return t.Test }},
		{"coverage", PhaseRunners, func(t config.Tools) bool { return t.Coverage }},
		{"mutants", PhaseRunners, func(t config.Tools) bool { return t.Mutants }},
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
