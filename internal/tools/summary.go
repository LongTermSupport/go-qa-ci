package tools

import (
	"context"

	"github.com/LongTermSupport/go-qa-ci/internal/defences"
	"github.com/LongTermSupport/go-qa-ci/internal/pipeline"
	"github.com/LongTermSupport/go-qa-ci/internal/summary"
)

// runSummary fails when the generated region in the agent instruction file is not what
// `go-qa summary` would write now. Without a configured file the check is off, and says so.
func (e Executor) runSummary() pipeline.Outcome {
	file := e.Ctx.Config.Summary.File
	if file == "" {
		if outcome, failed := e.report("summary: no [summary] file is configured (summary.file); the summary check is off"); failed {
			return outcome
		}
		return pipeline.Passed()
	}
	listing, err := defences.Collect(context.Background(), defences.Input{
		Root: e.Ctx.Root, Config: e.Ctx.Config, Steps: Steps(e.Ctx.Config),
	})
	if err != nil {
		return e.fail("%v", err)
	}
	f, stale, err := summary.Check(e.Ctx.Root, file, listing)
	if err != nil {
		return e.fail("%v", err)
	}
	if stale {
		return e.fail("%s", f)
	}
	return pipeline.Passed()
}
