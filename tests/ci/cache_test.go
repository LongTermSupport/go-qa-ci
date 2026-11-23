package ci_test

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"go.yaml.in/yaml/v3"
)

// defaultCacheKey is the file actions/setup-go hashes for its cache key when the step names none.
const defaultCacheKey = "go.sum"

// setupGoCacheProblems is the defence for the class "a QA runner makes Go redo work whose inputs
// did not change", in CI: an actions/setup-go step that caches (its default) keys the cache on the
// files cache-dependency-path names, else on go.sum at the repository root. When none of them
// exists, setup-go has nothing to key on: the module and build caches are not restored, and every
// run downloads the modules and rebuilds from cold. Each pattern must match a file in repo; a
// pattern with ** is not judged (fs.Glob cannot match it), which is stated rather than passed.
func setupGoCacheProblems(repo fs.FS, name string, w workflow) []string {
	var problems []string
	for jobName, j := range w.Jobs {
		for i, s := range j.Steps {
			if !strings.HasPrefix(s.Uses, "actions/setup-go@") || s.With["cache"] == false {
				continue
			}
			patterns := []string{defaultCacheKey}
			if value, set := s.With["cache-dependency-path"]; set {
				patterns = strings.Fields(fmt.Sprint(value))
			}
			for _, pattern := range patterns {
				if strings.Contains(pattern, "**") {
					problems = append(problems, fmt.Sprintf("%s job %s step %d: cache-dependency-path %q uses **, which this check cannot match; name the go.sum files", name, jobName, i, pattern))
					continue
				}
				matches, err := fs.Glob(repo, strings.TrimPrefix(pattern, "./"))
				if err != nil || len(matches) == 0 {
					problems = append(problems, fmt.Sprintf("%s job %s step %d: setup-go keys its cache on %q, which matches no file, so nothing is restored; "+
						"set cache-dependency-path to the module's go.sum (for a module in sub/, sub/go.sum)", name, jobName, i, pattern))
				}
			}
		}
	}
	return problems
}

func parseWorkflow(t *testing.T, text string) workflow {
	t.Helper()
	var w workflow
	if err := yaml.Unmarshal([]byte(text), &w); err != nil {
		t.Fatal(err)
	}
	return w
}

const setupGo = "jobs:\n  go:\n    steps:\n      - uses: actions/setup-go@v7\n        with:\n          go-version-file: sub/go.mod\n"

// Red and green on a fixture: a module in sub/ with setup-go left at its default key is the
// defect; naming sub/go.sum, or switching the cache off, is not.
func TestSetupGoCacheKeyMustNameAGoSumThatExists(t *testing.T) {
	repo := fstest.MapFS{"sub/go.mod": {}, "sub/go.sum": {}}
	if problems := setupGoCacheProblems(repo, "fixture.yml", parseWorkflow(t, setupGo)); len(problems) != 1 || !strings.Contains(problems[0], `"go.sum"`) {
		t.Errorf("the default key on a nested module: got %q", problems)
	}
	if problems := setupGoCacheProblems(repo, "fixture.yml", parseWorkflow(t, setupGo+"          cache-dependency-path: missing/go.sum\n")); len(problems) != 1 {
		t.Errorf("a key that names no file: got %q", problems)
	}
	for _, with := range []string{"          cache-dependency-path: sub/go.sum\n", "          cache-dependency-path: |\n            sub/go.sum\n            */go.sum\n", "          cache: false\n"} {
		if problems := setupGoCacheProblems(repo, "fixture.yml", parseWorkflow(t, setupGo+with)); len(problems) != 0 {
			t.Errorf("%q: got %q", with, problems)
		}
	}
}

func TestEverySetupGoStepKeysItsCacheOnAGoSumThatExists(t *testing.T) {
	repoRoot := filepath.Join("..", "..")
	workflows, err := os.OpenRoot(filepath.Join(repoRoot, ".github", "workflows"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := workflows.Close(); closeErr != nil {
			t.Error(closeErr)
		}
	}()
	entries, err := fs.ReadDir(workflows.FS(), ".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		data, readErr := workflows.ReadFile(entry.Name())
		if readErr != nil {
			t.Fatal(readErr)
		}
		for _, problem := range setupGoCacheProblems(os.DirFS(repoRoot), entry.Name(), parseWorkflow(t, string(data))) {
			t.Error(problem)
		}
	}
}
