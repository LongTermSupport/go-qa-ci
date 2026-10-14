package tools

import (
	"context"
	"path"
	"path/filepath"

	"github.com/LongTermSupport/go-qa-ci/internal/finding"
	"github.com/LongTermSupport/go-qa-ci/internal/pipeline"
	"github.com/LongTermSupport/go-qa-ci/internal/record"
	"github.com/LongTermSupport/go-qa-ci/internal/root"
	"github.com/LongTermSupport/go-qa-ci/internal/vet"
)

// runAnalyzers runs the project's go/analysis analyzers (the bundled ones and those in the
// project driver) through go vet, keeps the findings inside the scope that the project record
// does not license, and prints the rest compiler-style under the analyzer name. go vet -json
// exits 0 whatever it finds, so this decides the outcome. On a run over the whole project it also
// fails the record's findings exceptions for analyzer names that licensed nothing.
func (e Executor) runAnalyzers() pipeline.Outcome {
	rec, err := record.Load(filepath.Join(e.Ctx.Root, filepath.FromSlash(e.Ctx.Config.Record.Path)))
	if err != nil {
		return e.fail("%s: %v", e.Ctx.Config.Record.Path, err)
	}
	ctx := context.Background()
	tool, cleanup, err := vet.Resolve(ctx, e.Ctx.Root, e.Ctx.Config.Analyzers.Driver)
	if err != nil {
		return e.fail("%v", err)
	}
	defer cleanup()
	registered, err := tool.Analyzers(ctx)
	if err != nil {
		return e.fail("%v", err)
	}
	matcher := rec.Matcher()
	var reported []finding.Finding
	for _, module := range e.modules() {
		packages, run := e.packagesOf(module)
		if !run {
			continue
		}
		found, vetErr := tool.Vet(ctx, filepath.Join(e.Ctx.Root, filepath.FromSlash(module)), packages...)
		if vetErr != nil {
			return e.fail("analyzers in %s: %v", displayModule(module), vetErr)
		}
		for _, f := range found {
			f.Path = path.Join(module, f.Path)
			if !root.InScope(f.Path, e.Ctx.Scope) {
				continue
			}
			if _, licensed := matcher.Finding(f.Rule, f.Path); licensed {
				continue
			}
			reported = append(reported, f)
		}
	}
	if e.wholeProject() {
		names := make([]string, len(registered))
		for i, analyzer := range registered {
			names[i] = analyzer.Name
		}
		reported = append(reported, matcher.StaleFindingsOf(names, e.Ctx.Config.Record.Path)...)
	}
	return e.reportAll(reported)
}
