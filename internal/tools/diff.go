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

// measuredOf is the package patterns whose statements coverage counts in module, and whether
// there is anything to count. A full or widened run counts the packages it tests (no patterns); a
// narrowed run counts only the changed packages, so a change is not judged on the gaps of
// importers it never touched, and counts nothing in a module that holds only importers.
func (e Executor) measuredOf(module string) (patterns []string, measure bool) {
	if !e.Ctx.Diff.Narrow() {
		return nil, true
	}
	patterns = e.Ctx.Diff.Changed[module]
	return patterns, len(patterns) > 0
}

// runNarrowed narrows a step of a diff-mode run, or reports that the step is not narrowed:
//
//   - file-scoped detectors (fmt, suppression, ast-grep, stages that take {files}) run on the
//     changed files, and are skipped when there are none;
//   - per-package engines (golangci, analyzers, build, test, coverage) run on the changed packages and
//     their importers, and coverage counts the statements of the changed packages alone; deadcode and mutants, which are whole-program, run when any package is
//     affected;
//   - vuln and mod judge the whole module, not a package: a changed Go file can call a vulnerable
//     function of a module already required, or import one go.mod does not yet list, so they run
//     whenever any package is affected and are skipped only when no Go package changed;
//   - the repo-wide defences (record, docs, summary) are not narrowed.
func (e Executor) runNarrowed(step pipeline.Step) (outcome pipeline.Outcome, handled bool) {
	plan := e.Ctx.Diff
	switch step.Name {
	case "fmt":
		return e.onFiles(step, plan.GoFiles(), "no changed Go files"), true
	case "suppression", "ast-grep":
		return e.onFiles(step, plan.Scope, "no changed files"), true
	case "golangci", "analyzers", "build", "test", "coverage", "deadcode", "mutants", "vuln", "mod":
		if len(plan.Packages) == 0 {
			return pipeline.Skipped("no changed Go packages"), true
		}
		return pipeline.Outcome{}, false
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
