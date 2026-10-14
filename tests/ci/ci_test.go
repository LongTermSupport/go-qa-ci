// Package ci_test holds the CI workflow to what the documentation says it does: the pinned tools
// come from tools.lock, the self-gate runs with the history the record ratchet needs, and the
// workflows and scripts are linted.
package ci_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

type step struct {
	Run  string         `yaml:"run"`
	Uses string         `yaml:"uses"`
	With map[string]any `yaml:"with"`
}

type job struct {
	Steps []step `yaml:"steps"`
}

type workflow struct {
	Jobs map[string]job `yaml:"jobs"`
}

func readWorkflow(t *testing.T) (workflow, string) {
	t.Helper()
	root, err := os.OpenRoot(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := root.Close(); closeErr != nil {
			t.Error(closeErr)
		}
	}()
	data, err := root.ReadFile(".github/workflows/ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	var w workflow
	if err := yaml.Unmarshal(data, &w); err != nil {
		t.Fatal(err)
	}
	return w, string(data)
}

// runs reports whether some step of the job has a run script line containing every fragment.
func (j job) runs(fragments ...string) bool {
	for _, s := range j.Steps {
		for line := range strings.SplitSeq(s.Run, "\n") {
			ok := true
			for _, f := range fragments {
				ok = ok && strings.Contains(line, f)
			}
			if ok {
				return true
			}
		}
	}
	return false
}

func jobNamed(t *testing.T, w workflow, name string) job {
	t.Helper()
	j, ok := w.Jobs[name]
	if !ok {
		t.Fatalf("ci.yml has no job %q", name)
	}
	return j
}

func TestGoJobRunsFormatVetTestAndLint(t *testing.T) {
	w, _ := readWorkflow(t)
	j := jobNamed(t, w, "go")
	for _, want := range [][]string{{"gofmt -l"}, {"go vet ./..."}, {"go test", "-race", "./..."}, {"golangci-lint run ./..."}} {
		if !j.runs(want...) {
			t.Errorf("job go has no step running %v", want)
		}
	}
}

func TestGoJobInstallsItsToolsFromTheLock(t *testing.T) {
	w, _ := readWorkflow(t)
	if !jobNamed(t, w, "go").runs("scripts/install-tools.sh", "golangci-lint", "ast-grep") {
		t.Error("job go must install golangci-lint and ast-grep (whose tests run the real binary) with scripts/install-tools.sh")
	}
}

func TestSelfGateInstallsEveryPinnedToolBuildsAndRunsTheGate(t *testing.T) {
	w, _ := readWorkflow(t)
	j := jobNamed(t, w, "self-gate")
	if !j.runs("scripts/install-tools.sh") {
		t.Error("self-gate must run scripts/install-tools.sh")
	}
	if !j.runs("go install ./cmd/go-qa") {
		t.Error("self-gate must install the binary with go install ./cmd/go-qa")
	}
	if !j.runs("go-qa run --ci") {
		t.Error("self-gate must run go-qa run --ci")
	}
}

func TestSelfGateChecksOutTheFullHistory(t *testing.T) {
	w, _ := readWorkflow(t)
	for _, s := range jobNamed(t, w, "self-gate").Steps {
		if strings.HasPrefix(s.Uses, "actions/checkout@") {
			if s.With["fetch-depth"] != 0 {
				t.Errorf("self-gate checkout must set fetch-depth: 0 for the record ratchet, got %v", s.With["fetch-depth"])
			}
			return
		}
	}
	t.Error("self-gate has no checkout step")
}

func TestWorkflowsAndShellScriptsAreLinted(t *testing.T) {
	w, _ := readWorkflow(t)
	var found bool
	for _, j := range w.Jobs {
		if j.runs("actionlint") && j.runs("shellcheck", "scripts/*.sh") {
			found = true
		}
	}
	if !found {
		t.Error("a job must run actionlint and shellcheck over scripts/*.sh")
	}
}

func TestNoPinnedToolIsInstalledOutsideTheLock(t *testing.T) {
	_, text := readWorkflow(t)
	for _, forbidden := range []string{"go install golang.org/x/tools/cmd/deadcode", "go install golang.org/x/vuln", "GOLANGCI_LINT_VERSION"} {
		if strings.Contains(text, forbidden) {
			t.Errorf("ci.yml pins a tool itself (%q); the versions live in tools.lock", forbidden)
		}
	}
}
