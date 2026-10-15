package tools

import (
	"context"
	"fmt"
	"io"

	"github.com/LongTermSupport/go-qa-ci/internal/astgrep"
	"github.com/LongTermSupport/go-qa-ci/internal/docs"
	"github.com/LongTermSupport/go-qa-ci/internal/pipeline"
	"github.com/LongTermSupport/go-qa-ci/internal/record"
)

// Executor runs the steps of one run. A step no engine is built for fails with Pending's
// reason, so it is never reported as passing.
type Executor struct {
	// Ctx is the project and invocation the run belongs to.
	Ctx Context
	// Stderr receives the findings and diagnostics of the steps.
	Stderr io.Writer
	// Pack is the bundled ast-grep rule pack; nil means the pack shipped inside the binary.
	Pack *astgrep.Pack
	// Shared, when set, makes the test step measure coverage in the same run and hand it to the
	// coverage step; nil means each step runs on its own.
	Shared *Measurements
}

// Run executes one step; in a narrowed diff-mode run the step first narrows to what changed.
func (e Executor) Run(step pipeline.Step) pipeline.Outcome {
	if e.Ctx.Diff.Narrow() {
		if outcome, handled := e.runNarrowed(step); handled {
			return outcome
		}
	}
	return e.run(step)
}

// run dispatches a step to its engine.
func (e Executor) run(step pipeline.Step) pipeline.Outcome {
	switch step.Name {
	case "fmt":
		return e.runFmt()
	case "record":
		return e.runRecord()
	case "suppression":
		return e.runSuppression()
	case "summary":
		return e.runSummary()
	case "docs":
		return e.runDocs()
	case "ast-grep":
		return e.runAstGrep()
	case "golangci":
		return e.runGolangci()
	case "analyzers":
		return e.runAnalyzers()
	case "deadcode":
		return e.runDeadcode()
	case "vuln":
		return e.runVuln()
	case "mod":
		return e.runMod()
	case "build":
		return e.runBuild()
	case "test":
		return e.runTest()
	case "coverage":
		return e.runCoverage()
	case "mutants":
		return e.runMutants()
	default:
		if s, found := e.stageNamed(step.Name); found {
			return e.runStage(s)
		}
		return Pending{}.Run(step)
	}
}

// runDocs checks that every identifier resolves and every page belongs to one.
func (e Executor) runDocs() pipeline.Outcome {
	findings, err := docs.Run(context.Background(), e.Ctx.Root, e.Ctx.Config)
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

// runRecord validates the project record and its ratchet. A shrunk record has its ceiling lowered
// in place outside CI; in CI the unlowered ceiling fails. Staleness is not judged here: it needs every detector
// that applies the record to have run, so it is reported by the run that owns them.
func (e Executor) runRecord() pipeline.Outcome {
	report, err := record.Gate(e.Ctx.Root, e.Ctx.Config.Record, e.Ctx.Today, !e.Ctx.CI)
	if err != nil {
		return e.fail("%v", err)
	}
	for _, note := range report.Notes {
		if outcome, failed := e.report("%s", note); failed {
			return outcome
		}
	}
	findings := report.Findings
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
