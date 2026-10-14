package tools

import (
	"github.com/LongTermSupport/go-qa-ci/internal/pipeline"
)

// wholeProject reports whether the run judges the whole project, so a check that needs every
// finding of a detector (a record exception that licensed nothing is stale) may speak. A run
// limited by -p, or narrowed to what changed, cannot.
func (e Executor) wholeProject() bool {
	return len(e.Ctx.Scope) == 0 && !e.Ctx.Diff.Narrow()
}

// packagesOf is the package patterns to run in module and whether to run there at all. A full
// run, or a widened one, runs every package (no patterns); a narrowed run runs the changed
// packages and their importers, and nothing in a module none of them is in.
func (e Executor) packagesOf(module string) (patterns []string, run bool) {
	if !e.Ctx.Diff.Narrow() {
		return nil, true
	}
	patterns = e.Ctx.Diff.Packages[module]
	return patterns, len(patterns) > 0
}

// runNarrowed narrows a step of a diff-mode run, or reports that the step is not narrowed:
//
//   - file-scoped detectors (fmt, suppression, ast-grep, stages that take {files}) run on the
//     changed files, and are skipped when there are none;
//   - per-package engines (golangci, analyzers, test, coverage) run on the changed packages and
//     their importers; deadcode and mutants, which are whole-program, run when any package is
//     affected;
//   - vuln and mod judge go.mod and go.sum, whose change widens the run, so they are skipped;
//   - the repo-wide defences (record, docs, summary) are not narrowed.
func (e Executor) runNarrowed(step pipeline.Step) (outcome pipeline.Outcome, handled bool) {
	plan := e.Ctx.Diff
	switch step.Name {
	case "fmt":
		return e.onFiles(step, plan.GoFiles(), "no changed Go files"), true
	case "suppression", "ast-grep":
		return e.onFiles(step, plan.Scope, "no changed files"), true
	case "golangci", "analyzers", "test", "coverage", "deadcode", "mutants":
		if len(plan.Packages) == 0 {
			return pipeline.Skipped("no changed Go packages"), true
		}
		return pipeline.Outcome{}, false
	case "vuln", "mod":
		return pipeline.Skipped("go.mod and go.sum are unchanged"), true
	case "record", "docs", "summary":
		return pipeline.Outcome{}, false
	}
	if s, found := e.stageNamed(step.Name); found && s.TakesFiles(true) {
		return e.onFiles(step, plan.Scope, "no changed files"), true
	}
	return pipeline.Outcome{}, false
}

// onFiles runs the step limited to files, or skips it, saying why, when there are none.
func (e Executor) onFiles(step pipeline.Step, files []string, none string) pipeline.Outcome {
	if len(files) == 0 {
		return pipeline.Skipped(none)
	}
	scoped := e
	scoped.Ctx.Scope = files
	return scoped.run(step)
}
