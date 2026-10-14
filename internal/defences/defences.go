// Package defences enumerates the defences a project's configuration has active: the built-ins of
// the enabled tools, the linters golangci-lint enables, the analyzers of the vet tool, the
// ast-grep rules, and the project's own stages and the rules they print, beside the project record
// and its budget (TOOLING-SPEC 5.1, 5.2, 6.3). The listing is derived on every call from the
// configuration and the tools themselves; nothing in it is written by hand, so adding a rule to
// the configuration is the only edit that puts it in the listing.
package defences

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/LongTermSupport/go-qa-ci/internal/astgrep"
	"github.com/LongTermSupport/go-qa-ci/internal/builtin"
	"github.com/LongTermSupport/go-qa-ci/internal/config"
	"github.com/LongTermSupport/go-qa-ci/internal/docs"
	"github.com/LongTermSupport/go-qa-ci/internal/golangci"
	"github.com/LongTermSupport/go-qa-ci/internal/record"
	"github.com/LongTermSupport/go-qa-ci/internal/stagerun"
	"github.com/LongTermSupport/go-qa-ci/internal/vet"
)

// The engines a defence is routed through.
const (
	EngineBuiltin  = "builtin"
	EngineGolangci = "golangci"
	EngineAnalyzer = "analyzer"
	EngineAstGrep  = "ast-grep"
	EngineStage    = "stage"
)

// Where a defence comes from: shipped with go-qa, or written by the project.
const (
	OriginBundled = "bundled"
	OriginProject = "project"
)

// levelBlocking is the level of every defence that is not an ast-grep rule of a lower severity:
// all of them block (TOOLING-SPEC 4.5).
const levelBlocking = "error"

// Step is one step of the gate as the listing reports it.
type Step struct {
	// Name is the `-t` name.
	Name string `json:"name"`
	// Phase is format, detectors or runners.
	Phase string `json:"phase"`
	// Project is true for a stage the project declared, false for a built-in tool.
	Project bool `json:"project"`
	// Enabled reports whether a plain run executes it.
	Enabled bool `json:"enabled"`
}

// Defence is one rule a Defence-routing engine enforces.
type Defence struct {
	// ID is the identifier exactly as findings print it.
	ID string `json:"id"`
	// Engine is the tool that enforces it.
	Engine string `json:"engine"`
	// Origin says whether go-qa ships it or the project wrote it.
	Origin string `json:"origin"`
	// Phase is the phase of the step that enforces it.
	Phase string `json:"phase"`
	// Level is error for a defence that blocks; a lower severity an ast-grep rule declares.
	Level string `json:"level"`
	// Description says what it flags.
	Description string `json:"description"`
	// Fix is the first paragraph of the page's fix section, or the stage's own fix text.
	Fix string `json:"fix,omitempty"`
	// Doc is the command that prints the defence's full documentation (TOOLING-SPEC 5.1); empty
	// when nothing documents it yet, which the docs detector reports as a dangling identifier.
	Doc string `json:"doc,omitempty"`
	// Stage names the project stage that reports the rule; empty otherwise.
	Stage string `json:"stage,omitempty"`
}

// Listing is everything `go-qa rules` reports.
type Listing struct {
	// Defences are the active defences, grouped by engine.
	Defences []Defence `json:"defences"`
	// Stages are the format and detector steps in execution order.
	Stages []Step `json:"stages"`
	// Runners are the runner steps in execution order.
	Runners []Step `json:"runners"`
	// Record are the project record's exceptions.
	Record []record.ListedException `json:"record"`
	// Budget is the record against its caps.
	Budget record.Budget `json:"budget"`
}

// Input is what the listing is derived from.
type Input struct {
	// Root is the project root.
	Root string
	// Config is the effective configuration.
	Config config.Config
	// Steps are the gate's steps in execution order, tools and project stages.
	Steps []Step
}

// owners maps a built-in identifier prefix to the tool that raises it. The analyzer
// gqaci_sensitive_format is deliberately absent: it is listed with the analyzers of the vet tool,
// where it is active only when that tool carries it.
var owners = []struct{ prefix, tool string }{
	{"gqaci.record.", "record"},
	{"gqaci.suppression.", "suppression"},
	{"gqaci.summary.", "summary"},
	{"gqaci.docs.", "docs"},
	{"gqaci.rules.", "ast-grep"},
}

// Collect derives the listing. A tool that cannot report its rules (golangci-lint missing, an
// analyzer driver that does not build, a stage whose rules command fails) fails the listing: a
// partial one would pass for complete.
func Collect(ctx context.Context, in Input) (Listing, error) {
	listing := Listing{Stages: []Step{}, Runners: []Step{}}
	for _, step := range in.Steps {
		if step.Phase == "runners" {
			listing.Runners = append(listing.Runners, step)
		} else {
			listing.Stages = append(listing.Stages, step)
		}
	}
	collectors := []func(context.Context, Input) ([]Defence, error){builtins, golangciLinters, analyzers, astGrepRules, stageRules}
	for _, collector := range collectors {
		found, err := collector(ctx, in)
		if err != nil {
			return Listing{}, err
		}
		listing.Defences = append(listing.Defences, found...)
	}
	if err := quoteFixes(ctx, in, listing.Defences); err != nil {
		return Listing{}, err
	}
	rec, err := record.Load(filepath.Join(in.Root, filepath.FromSlash(in.Config.Record.Path)))
	if err != nil {
		return Listing{}, fmt.Errorf("%s: %w", in.Config.Record.Path, err)
	}
	listing.Record = rec.Entries()
	listing.Budget = record.BudgetOf(rec, in.Config.Record)
	return listing, nil
}

// step finds a step by name.
func (in Input) step(name string) (Step, error) {
	for _, s := range in.Steps {
		if s.Name == name {
			return s, nil
		}
	}
	return Step{}, fmt.Errorf("the listing needs the %s tool, which the gate's steps do not include", name)
}

// enabledStep is the step named, and whether it is switched on.
func (in Input) enabledStep(name string) (Step, bool, error) {
	s, err := in.step(name)
	return s, err == nil && s.Enabled, err
}

func builtins(_ context.Context, in Input) ([]Defence, error) {
	var out []Defence
	for _, id := range builtin.All() {
		if id == builtin.SensitiveFormat {
			continue
		}
		tool := ""
		for _, owner := range owners {
			if strings.HasPrefix(id, owner.prefix) {
				tool = owner.tool
			}
		}
		if tool == "" {
			return nil, fmt.Errorf("built-in %s has no owning tool; add its prefix to defences.owners", id)
		}
		s, enabled, err := in.enabledStep(tool)
		if err != nil {
			return nil, err
		}
		if enabled {
			out = append(out, Defence{ID: id, Engine: EngineBuiltin, Origin: OriginBundled, Phase: s.Phase, Level: levelBlocking, Description: builtin.Describe(id)})
		}
	}
	return out, nil
}

func golangciLinters(ctx context.Context, in Input) ([]Defence, error) {
	s, enabled, err := in.enabledStep("golangci")
	if err != nil || !enabled {
		return nil, err
	}
	runner, err := golangci.Locate()
	if err != nil {
		return nil, fmt.Errorf("listing the linters: %w", err)
	}
	configPath, cleanup, err := golangci.Config(in.Root)
	if err != nil {
		return nil, fmt.Errorf("listing the linters: %w", err)
	}
	defer cleanup()
	linters, err := runner.Linters(ctx, in.Root, configPath)
	if err != nil {
		return nil, fmt.Errorf("listing the linters: %w", err)
	}
	origin := OriginBundled
	if configPath == "" {
		origin = OriginProject
	}
	out := make([]Defence, len(linters))
	for i, l := range linters {
		out[i] = Defence{ID: l.Name, Engine: EngineGolangci, Origin: origin, Phase: s.Phase, Level: levelBlocking, Description: oneLine(l.Description)}
	}
	return out, nil
}

func analyzers(ctx context.Context, in Input) ([]Defence, error) {
	s, enabled, err := in.enabledStep("analyzers")
	if err != nil || !enabled {
		return nil, err
	}
	origin := OriginBundled
	if in.Config.Analyzers.Driver != "" {
		origin = OriginProject
	}
	var out []Defence
	err = vet.WithAnalyzers(ctx, in.Root, in.Config.Analyzers.Driver, func(_ vet.Tool, found []vet.Analyzer) error {
		for _, a := range found {
			out = append(out, Defence{ID: a.Name, Engine: EngineAnalyzer, Origin: origin, Phase: s.Phase, Level: levelBlocking, Description: oneLine(a.Summary)})
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("listing the analyzers: %w", err)
	}
	return out, nil
}

func astGrepRules(_ context.Context, in Input) ([]Defence, error) {
	s, enabled, err := in.enabledStep("ast-grep")
	if err != nil || !enabled {
		return nil, err
	}
	var out []Defence
	add := func(origin string, rules []astgrep.Rule) {
		for _, r := range rules {
			out = append(out, Defence{ID: r.ID, Engine: EngineAstGrep, Origin: origin, Phase: s.Phase, Level: r.Severity, Description: oneLine(r.Message)})
		}
	}
	if in.Config.AstGrep.Bundled {
		pack, packErr := astgrep.Bundled()
		if packErr != nil {
			return nil, fmt.Errorf("listing the bundled ast-grep rules: %w", packErr)
		}
		add(OriginBundled, pack.Rules())
	}
	project, found, err := astgrep.LoadProject(in.Root)
	if err != nil {
		return nil, fmt.Errorf("listing the ast-grep rules: %w", err)
	}
	if !found {
		return out, nil
	}
	inventory, err := project.Inventory(in.Root)
	if err != nil {
		return nil, fmt.Errorf("listing the ast-grep rules: %w", err)
	}
	add(OriginProject, inventory.Rules)
	return out, nil
}

// stageRules lists every project stage under its name, then the rules its rules command prints.
func stageRules(ctx context.Context, in Input) ([]Defence, error) {
	var out []Defence
	for _, s := range in.Config.Stages {
		out = append(out, Defence{
			ID: s.Name, Engine: EngineStage, Origin: OriginProject, Phase: s.Phase, Level: levelBlocking,
			Description: oneLine(s.Says), Fix: oneLine(s.Fix), Stage: s.Name,
		})
		rules, err := stagerun.LoadRules(ctx, in.Root, s)
		if err != nil {
			return nil, fmt.Errorf("listing the rules of stage %s: %w", s.Name, err)
		}
		for _, r := range rules {
			out = append(out, Defence{ID: r.ID, Engine: EngineStage, Origin: OriginProject, Phase: s.Phase, Level: levelBlocking, Description: oneLine(r.Description), Stage: s.Name})
		}
	}
	return out, nil
}

// golangciDocRoute prints every linter's documentation; golangci-lint has no per-linter explain.
const golangciDocRoute = "golangci-lint help linters"

// ruleDocRoute is the route of an identifier `go-qa rule-doc` resolves.
func ruleDocRoute(id string) string { return "go-qa rule-doc " + id }

// quoteFixes fills the documentation route of every defence and the fix text of the defences that do not carry their own from the first
// paragraph of the section the configuration names in the identifier's page. An identifier
// without a page keeps an empty fix; resolving it is the docs detector's business.
func quoteFixes(ctx context.Context, in Input, found []Defence) error {
	resolver := docs.ForConfig(in.Root, in.Config)
	for i, d := range found {
		doc, ok, err := resolver.Resolve(ctx, d.ID)
		if err != nil {
			return fmt.Errorf("finding the page of %s: %w", d.ID, err)
		}
		switch {
		case ok:
			found[i].Doc = ruleDocRoute(d.ID)
			if d.Fix == "" {
				found[i].Fix = docs.Fix(doc.Text, in.Config.Summary.FixHeadings)
			}
		case d.Engine == EngineAnalyzer:
			found[i].Doc = ruleDocRoute(d.ID)
		case d.Engine == EngineGolangci:
			found[i].Doc = golangciDocRoute
		}
	}
	return nil
}

// oneLine collapses whitespace, so a description is one line wherever it is printed.
func oneLine(text string) string {
	return strings.Join(strings.Fields(text), " ")
}
