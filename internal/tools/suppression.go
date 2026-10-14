package tools

import (
	"path/filepath"

	"github.com/LongTermSupport/go-qa-ci/internal/finding"
	"github.com/LongTermSupport/go-qa-ci/internal/pipeline"
	"github.com/LongTermSupport/go-qa-ci/internal/record"
	"github.com/LongTermSupport/go-qa-ci/internal/suppression"
)

// runSuppression scans the project for every route that silences a defence and fails each one the
// record does not license. On a run over the whole project it also fails the record exceptions
// that license no suppression: only this tool applies suppression exceptions, so it alone can
// judge them. A run limited to some paths cannot, and says nothing about them.
func (e Executor) runSuppression() pipeline.Outcome {
	rec, err := record.Load(filepath.Join(e.Ctx.Root, filepath.FromSlash(e.Ctx.Config.Record.Path)))
	if err != nil {
		return e.fail("%s: %v", e.Ctx.Config.Record.Path, err)
	}
	result, err := suppression.Scan(e.Ctx.Root, suppression.Options{Scope: e.Ctx.Scope, Exclude: e.Ctx.Config.ScanExclude})
	if err != nil {
		return e.fail("%v", err)
	}
	matcher := rec.Matcher()
	findings := suppression.Check(result, matcher)
	if len(e.Ctx.Scope) == 0 {
		findings = append(findings, matcher.StaleCovering(record.CoversSuppressions, e.Ctx.Config.Record.Path)...)
	}
	return e.reportAll(findings)
}

// reportAll prints the findings and fails the step when there are any.
func (e Executor) reportAll(findings []finding.Finding) pipeline.Outcome {
	for _, f := range findings {
		if outcome, failed := e.report("%s", f); failed {
			return outcome
		}
	}
	if len(findings) > 0 {
		return pipeline.Failed()
	}
	return pipeline.Passed()
}
