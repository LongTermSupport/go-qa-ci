package tools

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/LongTermSupport/go-qa-ci/internal/finding"
	"github.com/LongTermSupport/go-qa-ci/internal/golangci"
	"github.com/LongTermSupport/go-qa-ci/internal/pipeline"
	"github.com/LongTermSupport/go-qa-ci/internal/record"
	"github.com/LongTermSupport/go-qa-ci/internal/root"
	"github.com/LongTermSupport/go-qa-ci/internal/worktree"
)

// modules lists the directories of the Go modules a run covers, relative to the root and
// slash-separated: the root module ("") and the further modules the configuration names.
func (e Executor) modules() []string {
	return append([]string{""}, e.Ctx.Config.Modules...)
}

// modulePaths returns the scope entries that belong to module, relative to its directory, and
// whether the module is in scope at all. A nil slice with true means the whole module.
func (e Executor) modulePaths(module string) (paths []string, inScope bool) {
	if len(e.Ctx.Scope) == 0 {
		return nil, true
	}
	for _, entry := range e.Ctx.Scope {
		if entry == "." {
			return nil, true
		}
		if owner := e.owner(entry); owner == module {
			rel := strings.TrimPrefix(strings.TrimPrefix(entry, module), "/")
			if rel == "" {
				return nil, true
			}
			paths = append(paths, rel)
		}
	}
	return paths, len(paths) > 0
}

// owner is the module whose directory is the deepest one containing entry.
func (e Executor) owner(entry string) string {
	best := ""
	for _, module := range e.modules() {
		if module == "" || entry == module || strings.HasPrefix(entry, module+"/") {
			if len(module) >= len(best) {
				best = module
			}
		}
	}
	return best
}

// displayModule names a module directory for messages; the root module is ".".
func displayModule(module string) string {
	if module == "" {
		return "."
	}
	return module
}

// forEachModule runs fn in every module directory with the configuration that applies there.
func (e Executor) forEachModule(fn func(module, dir, configPath string) error) error {
	for _, module := range e.modules() {
		dir := filepath.Join(e.Ctx.Root, filepath.FromSlash(module))
		configPath, cleanup, err := golangci.Config(dir)
		if err != nil {
			return fmt.Errorf("configuration for %s: %w", displayModule(module), err)
		}
		err = fn(module, dir, configPath)
		cleanup()
		if err != nil {
			return err
		}
	}
	return nil
}

// runFmt applies the formatters. Outside CI it rewrites the files (or, with fixing off, fails with
// the diff); in CI it rewrites them and then fails because the working tree changed.
func (e Executor) runFmt() pipeline.Outcome {
	runner, err := golangci.Locate()
	if err != nil {
		return e.fail("%v", err)
	}
	check := !e.Ctx.Fix && !e.Ctx.CI
	var before map[string]string
	if e.Ctx.CI {
		if before, err = worktree.Snapshot(e.Ctx.Root); err != nil {
			return e.fail("%v", err)
		}
	}
	failed := false
	err = e.forEachModule(func(module, dir, configPath string) error {
		paths, inScope := e.modulePaths(module)
		if !inScope {
			return nil
		}
		diff, formatErr := runner.Format(context.Background(), dir, configPath, paths, check)
		if formatErr != nil {
			return fmt.Errorf("fmt in %s: %w", displayModule(module), formatErr)
		}
		if diff != "" {
			failed = true
			_, writeErr := fmt.Fprintf(e.Stderr, "error[fmt]: formatting would change these files; run go-qa without --no-fix\n%s\n", diff)
			return fmt.Errorf("writing the diff: %w", writeErr)
		}
		return nil
	})
	if err != nil {
		return e.fail("%v", err)
	}
	if e.Ctx.CI {
		after, snapErr := worktree.Snapshot(e.Ctx.Root)
		if snapErr != nil {
			return e.fail("%v", snapErr)
		}
		if changed := worktree.Changed(before, after); len(changed) > 0 {
			return e.fail("error[fmt]: working tree changed: formatting rewrote %s; commit the formatted code", strings.Join(changed, ", "))
		}
	}
	if failed {
		return pipeline.Failed()
	}
	return pipeline.Passed()
}

// runGolangci runs the aggregated linters, keeps the findings inside the scope that the project
// record does not license, and prints the rest compiler-style.
func (e Executor) runGolangci() pipeline.Outcome {
	runner, err := golangci.Locate()
	if err != nil {
		return e.fail("%v", err)
	}
	rec, err := record.Load(filepath.Join(e.Ctx.Root, filepath.FromSlash(e.Ctx.Config.Record.Path)))
	if err != nil {
		return e.fail("%s: %v", e.Ctx.Config.Record.Path, err)
	}
	matcher := rec.Matcher()
	var reported []finding.Finding
	err = e.forEachModule(func(module, dir, configPath string) error {
		found, lintErr := runner.Lint(context.Background(), dir, configPath)
		if lintErr != nil {
			return fmt.Errorf("lint in %s: %w", displayModule(module), lintErr)
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
		return nil
	})
	if err != nil {
		return e.fail("%v", err)
	}
	for _, f := range reported {
		if outcome, bad := e.report("%s", f); bad {
			return outcome
		}
	}
	if len(reported) > 0 {
		return pipeline.Failed()
	}
	return pipeline.Passed()
}
