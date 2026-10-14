package tools

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/LongTermSupport/go-qa-ci/internal/astgrep"
	"github.com/LongTermSupport/go-qa-ci/internal/builtin"
	"github.com/LongTermSupport/go-qa-ci/internal/finding"
	"github.com/LongTermSupport/go-qa-ci/internal/pipeline"
	"github.com/LongTermSupport/go-qa-ci/internal/record"
	"github.com/LongTermSupport/go-qa-ci/internal/root"
)

// astGrepPlan is what one run of the ast-grep detector covers.
type astGrepPlan struct {
	// project is the sgconfig.yml project; hasProject is false when there is none.
	project    astgrep.Project
	hasProject bool
	// inventory holds the project's rules.
	inventory astgrep.Inventory
	// bundled is the pack, and useBundled whether this run scans with it.
	bundled    astgrep.Pack
	useBundled bool
}

// ruleIDs are the identifiers of every rule this run hosts, the project's and the pack's.
func (p astGrepPlan) ruleIDs() []string {
	ids := make([]string, 0, len(p.inventory.Rules))
	for _, rule := range p.inventory.Rules {
		ids = append(ids, rule.ID)
	}
	if p.useBundled {
		ids = append(ids, p.bundled.IDs()...)
	}
	return ids
}

func (e Executor) astGrepPlan() (astGrepPlan, error) {
	var plan astGrepPlan
	var err error
	if e.Pack != nil {
		plan.bundled = *e.Pack
	} else if plan.bundled, err = astgrep.Bundled(); err != nil {
		return astGrepPlan{}, fmt.Errorf("loading the bundled pack: %w", err)
	}
	plan.useBundled = e.Ctx.Config.AstGrep.Bundled && !plan.bundled.Empty()
	if plan.project, plan.hasProject, err = astgrep.LoadProject(e.Ctx.Root); err != nil {
		return astGrepPlan{}, fmt.Errorf("loading the project: %w", err)
	}
	if plan.hasProject {
		if plan.inventory, err = plan.project.Inventory(e.Ctx.Root); err != nil {
			return astGrepPlan{}, fmt.Errorf("reading the project rules: %w", err)
		}
	}
	return plan, nil
}

// runAstGrep judges the project's ast-grep rules and runs them, with the bundled pack when it is
// on and not empty. The rules are judged without the binary: a rule with no id
// (gqaci.rules.unnamed) and an error-level rule without both a valid and an invalid test case
// (gqaci.rules.untested) fail the step whether ast-grep is installed or not. The scans then
// print every blocking match the project record does not license, and `ast-grep test` runs the
// project's own rule tests. Without the binary the step is skipped, saying why, once the rules
// themselves have passed. On a run over the whole project it also fails the record's findings
// exceptions for rules that licensed nothing.
func (e Executor) runAstGrep() pipeline.Outcome {
	plan, err := e.astGrepPlan()
	if err != nil {
		return e.fail("%v", err)
	}
	if !plan.hasProject && !plan.useBundled {
		return pipeline.Skipped("no sgconfig.yml and the bundled pack is empty or switched off")
	}
	rec, err := record.Load(filepath.Join(e.Ctx.Root, filepath.FromSlash(e.Ctx.Config.Record.Path)))
	if err != nil {
		return e.fail("%s: %v", e.Ctx.Config.Record.Path, err)
	}
	reported, err := e.ruleFindings(plan)
	if err != nil {
		return e.fail("%v", err)
	}
	runner, err := astgrep.Locate()
	if errors.Is(err, astgrep.ErrNotInstalled) {
		if len(reported) > 0 {
			return e.reportAll(reported)
		}
		return e.fail("%v; its rules were not run, and the pinned version is installed by scripts/install-tools.sh", err)
	}
	if err != nil {
		return e.fail("%v", err)
	}
	ctx := context.Background()
	matches, err := e.scan(ctx, runner, plan)
	if err != nil {
		return e.fail("%v", err)
	}
	matcher := rec.Matcher()
	reported = append(reported, e.unlicensed(matches, matcher)...)
	if len(e.Ctx.Scope) == 0 {
		reported = append(reported, matcher.StaleFindingsOf(plan.ruleIDs(), e.Ctx.Config.Record.Path)...)
	}
	outcome := e.reportAll(reported)
	if !plan.hasProject || !plan.project.HasTests() {
		return outcome
	}
	output, passed, err := runner.Test(ctx, e.Ctx.Root)
	if err != nil {
		return e.fail("%v", err)
	}
	if !passed {
		return e.fail("error[ast-grep test]: the project's rule tests failed\n%s", output)
	}
	return outcome
}

// ruleFindings are the findings about the rules themselves, which need no scan.
func (e Executor) ruleFindings(plan astGrepPlan) ([]finding.Finding, error) {
	if !plan.hasProject {
		return nil, nil
	}
	tested, err := plan.project.Tested(e.Ctx.Root)
	if err != nil {
		return nil, fmt.Errorf("reading the rule tests: %w", err)
	}
	var findings []finding.Finding
	for _, file := range plan.inventory.Unnamed {
		findings = append(findings, finding.Finding{
			Rule:    builtin.RulesUnnamed,
			Path:    file,
			Message: "a rule here has no `id`, so ast-grep reports it under the file's name, which nothing lists, documents or tests",
		})
	}
	for _, rule := range plan.inventory.Rules {
		if rule.Severity != "error" || slices.Contains(tested, rule.ID) {
			continue
		}
		findings = append(findings, finding.Finding{
			Rule:    builtin.RulesUntested,
			Path:    rule.File,
			Message: fmt.Sprintf("`%s` has no test case with both `valid` and `invalid` code in a testConfigs directory of %s", rule.ID, astgrep.ConfigFile),
		})
	}
	return findings, nil
}

// unlicensed are the blocking matches inside the scope that the project record does not license.
func (e Executor) unlicensed(matches []astgrep.Match, matcher *record.Matcher) []finding.Finding {
	var findings []finding.Finding
	for _, match := range matches {
		if !match.Blocking() || !root.InScope(match.Path, e.Ctx.Scope) {
			continue
		}
		if _, licensed := matcher.Finding(match.Rule, match.Path); licensed {
			continue
		}
		findings = append(findings, finding.Finding{Rule: match.Rule, Path: match.Path, Line: match.Line, Message: matchMessage(match)})
	}
	return findings
}

// scan runs the project scan and the bundled-pack scan and returns every match. The pack is
// written to a private temporary directory, so the scan leaves the project tree untouched.
func (e Executor) scan(ctx context.Context, runner astgrep.Runner, plan astGrepPlan) (matches []astgrep.Match, err error) {
	if plan.hasProject {
		projectMatches, scanErr := runner.Scan(ctx, e.Ctx.Root, astgrep.ScanOptions{Paths: e.Ctx.Scope})
		if scanErr != nil {
			return nil, fmt.Errorf("project rules: %w", scanErr)
		}
		matches = projectMatches
	}
	if !plan.useBundled {
		return matches, nil
	}
	dir, err := os.MkdirTemp("", "go-qa-ast-grep-")
	if err != nil {
		return nil, fmt.Errorf("creating a directory for the bundled pack: %w", err)
	}
	defer func() { err = errors.Join(err, os.RemoveAll(dir)) }()
	rule, err := plan.bundled.Write(dir)
	if err != nil {
		return nil, fmt.Errorf("writing the bundled pack: %w", err)
	}
	packMatches, err := runner.Scan(ctx, e.Ctx.Root, astgrep.ScanOptions{Rule: rule, Excludes: e.Ctx.Config.ScanExclude, Paths: e.Ctx.Scope})
	if err != nil {
		return nil, fmt.Errorf("bundled pack: %w", err)
	}
	return append(matches, packMatches...), nil
}

func matchMessage(match astgrep.Match) string {
	if match.Note == "" {
		return match.Message
	}
	return fmt.Sprintf("%s (%s)", match.Message, match.Note)
}
