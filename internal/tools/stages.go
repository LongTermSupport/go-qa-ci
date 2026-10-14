package tools

import (
	"context"
	"slices"
	"strings"

	"github.com/LongTermSupport/go-qa-ci/internal/finding"
	"github.com/LongTermSupport/go-qa-ci/internal/pipeline"
	"github.com/LongTermSupport/go-qa-ci/internal/root"
	"github.com/LongTermSupport/go-qa-ci/internal/runners"
	"github.com/LongTermSupport/go-qa-ci/internal/stage"
	"github.com/LongTermSupport/go-qa-ci/internal/stagerun"
)

// stageNamed is the project stage a step belongs to.
func (e Executor) stageNamed(name string) (stage.Stage, bool) {
	i := slices.IndexFunc(e.Ctx.Config.Stages, func(s stage.Stage) bool { return s.Name == name })
	if i < 0 {
		return stage.Stage{}, false
	}
	return e.Ctx.Config.Stages[i], true
}

// runStage runs a project stage. A stage whose command takes {files} and finds none is skipped,
// saying so. A stage with output = "status" passes on exit code 0; one with output = "findings"
// prints a JSON list that the project record is applied to, so a stage's rules are licensed,
// listed and documented like any other defence.
func (e Executor) runStage(s stage.Stage) pipeline.Outcome {
	scoped := len(e.Ctx.Scope) > 0
	var files []string
	if s.TakesFiles(scoped) {
		var err error
		if files, err = s.MatchingFiles(e.Ctx.Root, e.Ctx.Scope); err != nil {
			return e.fail("%v", err)
		}
	}
	argv, ok := s.Argv(scoped, files)
	if !ok {
		return pipeline.Skipped("no files for this stage")
	}
	out, err := stagerun.Capture(context.Background(), e.Ctx.Root, s, argv)
	if err != nil {
		return e.fail("%v", err)
	}
	if s.Output.Effective() == stage.OutputStatus {
		return e.statusOutcome(s, out)
	}
	return e.findingsOutcome(s, out)
}

// statusOutcome shows what the stage printed and lets its exit code decide.
func (e Executor) statusOutcome(s stage.Stage, out runners.Output) pipeline.Outcome {
	if text := out.Combined(); text != "" {
		if outcome, failed := e.report("%s", strings.TrimRight(text, "\n")); failed {
			return outcome
		}
	}
	if out.Code != 0 {
		return e.fail("error[%s]: the stage exited with code %d; its output is above", s.Name, out.Code)
	}
	return pipeline.Passed()
}

// findingsOutcome reads the findings a stage printed and fails on those the record does not
// license. Exit codes other than 0 and 1, an unreadable list, and exit code 1 with nothing reported
// all mean the stage did not do its job, which is never a pass.
func (e Executor) findingsOutcome(s stage.Stage, out runners.Output) pipeline.Outcome {
	const findingsExit = 1
	if out.Stderr != "" {
		if outcome, failed := e.report("%s", strings.TrimRight(out.Stderr, "\n")); failed {
			return outcome
		}
	}
	if out.Code != 0 && out.Code != findingsExit {
		return e.fail("error[%s]: the stage did not complete (exit code %d)", s.Name, out.Code)
	}
	found, err := stage.ParseFindings(out.Stdout, s.Dir)
	if err != nil {
		return e.fail("error[%s]: the stage printed no usable findings: %v", s.Name, err)
	}
	if out.Code == findingsExit && len(found) == 0 {
		return e.fail("error[%s]: the stage exited with code %d but reported no findings", s.Name, findingsExit)
	}
	rec, err := e.loadRecord()
	if err != nil {
		return e.fail("%v", err)
	}
	matcher := rec.Matcher()
	var reported []finding.Finding
	for _, f := range found {
		if !root.InScope(f.Path, e.Ctx.Scope) {
			continue
		}
		if _, licensed := matcher.Finding(f.Rule, f.Path); !licensed {
			reported = append(reported, f)
		}
	}
	if len(e.Ctx.Scope) == 0 {
		hosted, rulesErr := stagerun.LoadRules(context.Background(), e.Ctx.Root, s)
		if rulesErr != nil {
			return e.fail("%v", rulesErr)
		}
		ids := make([]string, len(hosted))
		for i, rule := range hosted {
			ids[i] = rule.ID
		}
		reported = append(reported, matcher.StaleFindingsOf(ids, e.Ctx.Config.Record.Path)...)
	}
	return e.reportAll(reported)
}
