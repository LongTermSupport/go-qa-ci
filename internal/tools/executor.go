package tools

import (
	"fmt"
	"io"

	"github.com/LongTermSupport/go-qa-ci/internal/pipeline"
	"github.com/LongTermSupport/go-qa-ci/internal/record"
)

// Executor runs the steps of one run. A tool whose engine is not built yet fails with Pending's
// reason, so it is never reported as passing.
type Executor struct {
	// Ctx is the project and invocation the run belongs to.
	Ctx Context
	// Stderr receives the findings and diagnostics of the steps.
	Stderr io.Writer
}

// Run executes one step.
func (e Executor) Run(step pipeline.Step) pipeline.Outcome {
	if step.Name == "record" {
		return e.runRecord()
	}
	return Pending{}.Run(step)
}

// runRecord validates the project record. Staleness is not judged here: it needs every detector
// that applies the record to have run, so it is reported by the run that owns them.
func (e Executor) runRecord() pipeline.Outcome {
	_, findings, err := record.Check(e.Ctx.Root, e.Ctx.Config.Record.Path)
	if err != nil {
		return e.fail("%v", err)
	}
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

// report writes a diagnostic line; failing to write it fails the step.
func (e Executor) report(format string, args ...any) (pipeline.Outcome, bool) {
	if _, err := fmt.Fprintf(e.Stderr, format+"\n", args...); err != nil {
		return pipeline.Outcome{Kind: pipeline.KindFailed, Reason: err.Error()}, true
	}
	return pipeline.Outcome{}, false
}

// fail writes a diagnostic line and fails the step.
func (e Executor) fail(format string, args ...any) pipeline.Outcome {
	if outcome, failed := e.report(format, args...); failed {
		return outcome
	}
	return pipeline.Failed()
}
