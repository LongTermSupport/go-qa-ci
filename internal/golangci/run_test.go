package golangci_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/configs"
	"github.com/LongTermSupport/go-qa-ci/internal/finding"
	"github.com/LongTermSupport/go-qa-ci/internal/golangci"
	"github.com/LongTermSupport/go-qa-ci/internal/tooltest"
)

// locate fails the test when golangci-lint is not installed: the tool is a required dependency,
// so its absence is a failure like that of every other pinned tool (internal/tooltest).
func locate(t *testing.T) golangci.Runner {
	t.Helper()
	runner, err := golangci.Locate()
	tooltest.Require(t, "golangci-lint", err)
	return runner
}

func module(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	files["go.mod"] = "module example.com/fixture\n\ngo 1.26\n"
	for name, text := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func rules(findings []finding.Finding) []string {
	out := make([]string, len(findings))
	for i, f := range findings {
		out[i] = f.Rule + "@" + f.Path
	}
	return out
}

func has(findings []finding.Finding, rule, path string) bool {
	for _, f := range findings {
		if f.Rule == rule && f.Path == path {
			return true
		}
	}
	return false
}

func readFile(t *testing.T, dir, name string) string {
	t.Helper()
	handle, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := handle.Close(); closeErr != nil {
			t.Error(closeErr)
		}
	}()
	data, err := handle.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestConfigDefersToTheProjectFile(t *testing.T) {
	dir := module(t, map[string]string{".golangci.yml": "version: \"2\"\n"})
	path, cleanup, err := golangci.Config(dir)
	defer cleanup()
	if err != nil || path != "" {
		t.Errorf("got %q, %v; want the project file to be used", path, err)
	}
}

func TestConfigWritesTheBaselineAndRemovesIt(t *testing.T) {
	dir := module(t, map[string]string{})
	path, cleanup, err := golangci.Config(dir)
	if err != nil {
		t.Fatal(err)
	}
	if written := readFile(t, filepath.Dir(path), filepath.Base(path)); written != string(configs.Golangci()) {
		t.Errorf("baseline not written faithfully: %q", written)
	}
	cleanup()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("baseline survived cleanup: %v", err)
	}
}

func TestBaselineIsAValidV2Configuration(t *testing.T) {
	runner := locate(t)
	dir := module(t, map[string]string{})
	path, cleanup, err := golangci.Config(dir)
	defer cleanup()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(context.Background(), runner.Binary, "config", "verify", "--config", path)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("config verify: %v\n%s", err, out)
	}
}

func TestBaselineHasNoExclusionsAndCapsNothing(t *testing.T) {
	text := string(configs.Golangci())
	for _, want := range []string{"max-issues-per-linter: 0", "max-same-issues: 0", "require-explanation: true", "require-specific: true", "allow-unused: false"} {
		if !strings.Contains(text, want) {
			t.Errorf("baseline lacks %q", want)
		}
	}
	for _, banned := range []string{"exclusions:", "new-from"} {
		if strings.Contains(text, banned) {
			t.Errorf("baseline carries %q", banned)
		}
	}
}

func lintFixture(t *testing.T, files map[string]string) []finding.Finding {
	t.Helper()
	runner := locate(t)
	dir := module(t, files)
	path, cleanup, err := golangci.Config(dir)
	defer cleanup()
	if err != nil {
		t.Fatal(err)
	}
	findings, err := runner.Lint(context.Background(), dir, path)
	if err != nil {
		t.Fatal(err)
	}
	return findings
}

func TestLintReportsErrcheckAndStaticcheckCodes(t *testing.T) {
	findings := lintFixture(t, map[string]string{
		"main.go":  "package main\n\nimport \"os\"\n\nfunc main() {\n\tos.Remove(\"x\")\n}\n",
		"sub/s.go": "// Package sub is a fixture.\npackage sub\n\nimport \"io/ioutil\"\n\n// Read reads.\nfunc Read() ([]byte, error) { return ioutil.ReadFile(\"x\") }\n",
	})
	if !has(findings, "errcheck", "main.go") {
		t.Errorf("no errcheck finding in %v", rules(findings))
	}
	if !has(findings, "staticcheck::SA1019", "sub/s.go") {
		t.Errorf("no staticcheck::SA1019 finding in %v", rules(findings))
	}
}

// TestLintRunsWhileAnotherGolangciLintHoldsTheLock holds the lock golangci-lint takes in its
// temporary directory, as an editor integration or a second go-qa run would, and expects the lint
// to proceed instead of waiting and failing with exit 3.
func TestLintRunsWhileAnotherGolangciLintHoldsTheLock(t *testing.T) {
	runner := locate(t)
	dir := module(t, map[string]string{
		"main.go": "// Package main is a fixture.\npackage main\n\nfunc main() {}\n",
	})
	path, cleanup, err := golangci.Config(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	root, err := os.OpenRoot(tmp)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := root.Close(); closeErr != nil {
			t.Error(closeErr)
		}
	}()
	lock, err := root.Create("golangci-lint.lock")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := lock.Close(); closeErr != nil {
			t.Error(closeErr)
		}
	}()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		t.Fatal(err)
	}

	if _, err := runner.Lint(context.Background(), dir, path); err != nil {
		t.Errorf("lint failed while the lock was held: %v", err)
	}
}

func TestLintOfACleanModuleIsEmpty(t *testing.T) {
	findings := lintFixture(t, map[string]string{
		"main.go": "// Package main is a fixture.\npackage main\n\nfunc main() {}\n",
	})
	if len(findings) != 0 {
		t.Errorf("got %v", rules(findings))
	}
}

func TestLintNeverPassesCodeThatDoesNotCompile(t *testing.T) {
	runner := locate(t)
	dir := module(t, map[string]string{"main.go": "package main\n\nfunc main() { undefined() }\n"})
	findings, err := runner.Lint(context.Background(), dir, "")
	if err == nil && len(findings) == 0 {
		t.Error("broken code passed with no finding and no error")
	}
}

const unformatted = "// Package main is a fixture.\npackage main\n\nfunc main() {\n\tprintln( \"hi\")\n}\n"

func TestFormatCheckReportsADiffAndFixRewrites(t *testing.T) {
	runner := locate(t)
	dir := module(t, map[string]string{"main.go": unformatted})
	path, cleanup, err := golangci.Config(dir)
	defer cleanup()
	if err != nil {
		t.Fatal(err)
	}
	diff, err := runner.Format(context.Background(), dir, path, nil, true)
	if err != nil || !strings.Contains(diff, "println(\"hi\")") {
		t.Fatalf("check: %q, %v", diff, err)
	}
	if got := readFile(t, dir, "main.go"); got != unformatted {
		t.Fatalf("check mode modified the file: %q", got)
	}
	if _, err := runner.Format(context.Background(), dir, path, nil, false); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, dir, "main.go"); strings.Contains(got, "( \"hi\"") {
		t.Errorf("fix left the file unformatted: %q", got)
	}
	if diff, err := runner.Format(context.Background(), dir, path, nil, true); err != nil || diff != "" {
		t.Errorf("formatted code still differs: %q, %v", diff, err)
	}
}
