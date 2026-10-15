// Package ci_test holds the CI workflow to what the documentation says it does: the pinned tools
// come from tools.lock, the self-gate runs with the history the record ratchet needs, and the
// workflows and scripts are linted.
package ci_test

import (
	"io/fs"
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
	if !jobNamed(t, w, "go").runs("scripts/install-tools.sh", "golangci-lint", "ast-grep", "gremlins") {
		t.Error("job go must install golangci-lint, ast-grep and gremlins (whose tests run the real binary and fail without it) with scripts/install-tools.sh")
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

// Every external tool a workflow downloads comes through scripts/install-tools.sh, which verifies it
// against the sha256 tools.lock pins; a workflow that downloads and checks a tool itself would
// trust the checksum file of the same release.
func TestNoWorkflowDownloadsAReleaseToolItself(t *testing.T) {
	workflows, err := os.OpenRoot(filepath.Join("..", "..", ".github", "workflows"))
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
		for _, forbidden := range []string{"gh release download", "checksums.txt", "sha256sum -c --ignore-missing", "ACTIONLINT_VERSION", "GITLEAKS_VERSION"} {
			if strings.Contains(string(data), forbidden) {
				t.Errorf("%s fetches or pins a release tool itself (%q); use scripts/install-tools.sh, which verifies the sha256 tools.lock pins", entry.Name(), forbidden)
			}
		}
	}
}

func TestWorkflowsInstallActionlintAndGitleaksFromTheLock(t *testing.T) {
	_, ciText := readWorkflow(t)
	if !strings.Contains(ciText, "scripts/install-tools.sh --print-path actionlint") {
		t.Error("a ci.yml job must install actionlint with scripts/install-tools.sh --print-path actionlint")
	}
	data, err := os.ReadFile(filepath.Join("..", "..", ".github", "workflows", "public-safety.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "scripts/install-tools.sh --print-path gitleaks") {
		t.Error("public-safety.yml must install gitleaks with scripts/install-tools.sh --print-path gitleaks")
	}
}

// patchedGoDirective is the first Go release carrying every standard library advisory fix that
// govulncheck reports against this module's own code; a lower toolchain turns the vuln runner red.
// The go directive, not a toolchain line, is what actions/setup-go reads from go.mod.
const patchedGoDirective = "go 1.26.9"

func TestEverySetupGoStepReadsTheToolchainFromGoMod(t *testing.T) {
	w, _ := readWorkflow(t)
	var seen int
	for name, j := range w.Jobs {
		for _, s := range j.Steps {
			if !strings.HasPrefix(s.Uses, "actions/setup-go@") {
				continue
			}
			seen++
			if s.With["go-version-file"] != "go.mod" {
				t.Errorf("job %s: setup-go must read the toolchain from go.mod, got %v", name, s.With)
			}
		}
	}
	if seen == 0 {
		t.Error("ci.yml has no setup-go step")
	}
}

func TestGoModRequiresThePatchedGo(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		if line == patchedGoDirective {
			return
		}
	}
	t.Errorf("go.mod must contain %q so CI and consumers build with the patched standard library", patchedGoDirective)
}
