package tools

import (
	"context"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/LongTermSupport/go-qa-ci/internal/config"
	"github.com/LongTermSupport/go-qa-ci/internal/finding"
	"github.com/LongTermSupport/go-qa-ci/internal/pipeline"
	"github.com/LongTermSupport/go-qa-ci/internal/record"
	"github.com/LongTermSupport/go-qa-ci/internal/root"
	"github.com/LongTermSupport/go-qa-ci/internal/runners"
)

// vulnIDPrefix starts every Go vulnerability database identifier; the record exceptions with it
// are the ones the vuln runner owns.
const vulnIDPrefix = "GO-"

// moduleDir is the directory of a module, from the root.
func (e Executor) moduleDir(module string) string {
	return filepath.Join(e.Ctx.Root, filepath.FromSlash(module))
}

// eachModuleInScope runs fn in every module the run's scope reaches. A scope that names only
// files of other modules leaves this one out; the runners themselves always cover a whole module.
func (e Executor) eachModuleInScope(fn func(module, dir string) error) error {
	for _, module := range e.modules() {
		if _, inScope := e.modulePaths(module); !inScope {
			continue
		}
		if err := fn(module, e.moduleDir(module)); err != nil {
			return err
		}
	}
	return nil
}

// unlicensedFindings runs scan in every module and keeps the findings inside the scope that the
// project record does not license, with module-relative paths made root-relative.
func (e Executor) unlicensedFindings(matcher *record.Matcher, scan func(ctx context.Context, dir string) ([]finding.Finding, error)) ([]finding.Finding, error) {
	var reported []finding.Finding
	for _, module := range e.modules() {
		found, err := scan(context.Background(), e.moduleDir(module))
		if err != nil {
			return nil, fmt.Errorf("in %s: %w", displayModule(module), err)
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
	return reported, nil
}

// loadRecord reads the project record.
func (e Executor) loadRecord() (record.Record, error) {
	rec, err := record.Load(filepath.Join(e.Ctx.Root, filepath.FromSlash(e.Ctx.Config.Record.Path)))
	if err != nil {
		return record.Record{}, fmt.Errorf("%s: %w", e.Ctx.Config.Record.Path, err)
	}
	return rec, nil
}

// runVuln fails every vulnerability whose vulnerable symbol the code calls and the project record
// does not license. On a run over the whole project it also fails the record's vulnerability
// exceptions that licensed nothing, so a fixed advisory leaves no exception behind.
func (e Executor) runVuln() pipeline.Outcome {
	rec, err := e.loadRecord()
	if err != nil {
		return e.fail("%v", err)
	}
	matcher := rec.Matcher()
	reported, err := e.unlicensedFindings(matcher, runners.Vuln)
	if err != nil {
		return e.fail("%v", err)
	}
	if len(e.Ctx.Scope) == 0 {
		var owned []string
		for _, exception := range rec.Exceptions {
			if strings.HasPrefix(strings.TrimSpace(exception.Rule), vulnIDPrefix) {
				owned = append(owned, strings.TrimSpace(exception.Rule))
			}
		}
		reported = append(reported, matcher.StaleFindingsOf(owned, e.Ctx.Config.Record.Path)...)
	}
	return e.reportAll(reported)
}

// runDeadcode fails the functions no main function or test reaches, in the modules that have a
// main package, minus those the project record licenses.
func (e Executor) runDeadcode() pipeline.Outcome {
	rec, err := e.loadRecord()
	if err != nil {
		return e.fail("%v", err)
	}
	matcher := rec.Matcher()
	applied := false
	reported, err := e.unlicensedFindings(matcher, func(ctx context.Context, dir string) ([]finding.Finding, error) {
		found, applies, runErr := runners.Deadcode(ctx, dir)
		applied = applied || applies
		if runErr != nil {
			return nil, fmt.Errorf("deadcode: %w", runErr)
		}
		return found, nil
	})
	if err != nil {
		return e.fail("%v", err)
	}
	if !applied {
		return pipeline.Skipped("no main package: deadcode starts from main functions, and the unused linter covers libraries")
	}
	if len(e.Ctx.Scope) == 0 {
		reported = append(reported, matcher.StaleFindingsOf([]string{runners.DeadcodeRule}, e.Ctx.Config.Record.Path)...)
	}
	return e.reportAll(reported)
}

// runMod fails a module whose go.mod or go.sum is not tidy, or whose dependencies do not verify.
func (e Executor) runMod() pipeline.Outcome {
	failed := false
	err := e.eachModuleInScope(func(module, dir string) error {
		problems, err := runners.Mod(context.Background(), dir)
		if err != nil {
			return fmt.Errorf("mod in %s: %w", displayModule(module), err)
		}
		for _, problem := range problems {
			failed = true
			if _, writeErr := fmt.Fprintf(e.Stderr, "error[mod]: %s: %s\n", displayModule(module), problem); writeErr != nil {
				return fmt.Errorf("writing the problem: %w", writeErr)
			}
		}
		return nil
	})
	return e.verdict(err, failed)
}

// runTest runs every module's tests with the race detector, shuffled and uncached.
func (e Executor) runTest() pipeline.Outcome {
	failed := false
	err := e.eachModuleInScope(func(module, dir string) error {
		output, passed, err := runners.Test(context.Background(), dir)
		if err != nil {
			return fmt.Errorf("test in %s: %w", displayModule(module), err)
		}
		if passed {
			return nil
		}
		failed = true
		if _, writeErr := fmt.Fprintf(e.Stderr, "error[test]: the tests failed in %s\n%s\n", displayModule(module), strings.TrimSpace(output)); writeErr != nil {
			return fmt.Errorf("writing the output: %w", writeErr)
		}
		return nil
	})
	return e.verdict(err, failed)
}

// runCoverage fails a module whose statement coverage is below the configured threshold.
func (e Executor) runCoverage() pipeline.Outcome {
	threshold := e.Ctx.Config.Coverage.FailUnderStatements
	failed := false
	err := e.eachModuleInScope(func(module, dir string) error {
		message, err := coverageProblem(dir, threshold)
		if err != nil {
			return fmt.Errorf("coverage in %s: %w", displayModule(module), err)
		}
		if message == "" {
			return nil
		}
		failed = true
		if _, writeErr := fmt.Fprintf(e.Stderr, "error[coverage]: %s: %s\n", displayModule(module), message); writeErr != nil {
			return fmt.Errorf("writing the problem: %w", writeErr)
		}
		return nil
	})
	return e.verdict(err, failed)
}

// coverageProblem measures the module and describes how it misses the threshold; it is empty when
// the threshold is met. Failing tests are a problem of the coverage gate too: nothing was measured.
func coverageProblem(dir string, threshold int) (string, error) {
	cov, err := runners.MeasureCoverage(context.Background(), dir)
	var testsFailed *runners.TestsFailedError
	if errors.As(err, &testsFailed) {
		return strings.TrimSpace(testsFailed.Error()), nil
	}
	if err != nil {
		return "", fmt.Errorf("measuring: %w", err)
	}
	if cov.Total >= float64(threshold) {
		return "", nil
	}
	message := fmt.Sprintf("statement coverage %.1f%% is below fail_under_statements = %d", cov.Total, threshold)
	if len(cov.Uncovered) > 0 {
		message += "\n  functions no test reaches:\n    " + strings.Join(cov.Uncovered, "\n    ")
	}
	return message, nil
}

// runMutants runs mutation testing in the configured mode and fails when the tests kill too few
// mutants. Mode off skips the step, saying so.
func (e Executor) runMutants() pipeline.Outcome {
	mutants := e.Ctx.Config.Mutants
	if mutants.Mode == config.MutantsOff {
		return pipeline.Skipped("mutants.mode is off")
	}
	opts := runners.MutantsOptions{Mode: mutants.Mode, Base: mutants.Base, ThresholdEfficacy: mutants.ThresholdEfficacy}
	failed := false
	err := e.eachModuleInScope(func(module, dir string) error {
		output, passed, err := runners.Mutants(context.Background(), dir, opts)
		if err != nil {
			return fmt.Errorf("mutants in %s: %w", displayModule(module), err)
		}
		if passed {
			return nil
		}
		failed = true
		if _, writeErr := fmt.Fprintf(e.Stderr, "error[mutants]: too few mutants were killed in %s (threshold_efficacy = %d)\n%s\n", displayModule(module), mutants.ThresholdEfficacy, strings.TrimSpace(output)); writeErr != nil {
			return fmt.Errorf("writing the output: %w", writeErr)
		}
		return nil
	})
	return e.verdict(err, failed)
}

// verdict turns the result of a per-module loop into an outcome: an error is reported and fails.
func (e Executor) verdict(err error, failed bool) pipeline.Outcome {
	if err != nil {
		return e.fail("%v", err)
	}
	if failed {
		return pipeline.Failed()
	}
	return pipeline.Passed()
}
