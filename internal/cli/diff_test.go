package cli_test

import (
	"os"
	goexec "os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	full := append([]string{"-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false", "-c", "init.defaultBranch=main"}, args...)
	cmd := goexec.CommandContext(t.Context(), "git", full...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func writeIn(t *testing.T, dir, rel, content string) {
	t.Helper()
	path := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// gitModule is a committed module with a leaf package, an importer of it and an unrelated package.
func gitModule(t *testing.T) string {
	t.Helper()
	dir := module(t, "")
	writeIn(t, dir, "leaf/leaf.go", "package leaf\n\nfunc Leaf() int { return 1 }\n")
	writeIn(t, dir, "mid/mid.go", "package mid\n\nimport \"example.com/m/leaf\"\n\nfunc Mid() int { return leaf.Leaf() }\n")
	writeIn(t, dir, "other/other.go", "package other\n\nfunc Other() int { return 2 }\n")
	gitIn(t, dir, "init")
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-m", "initial")
	return dir
}

func TestRunChangedNamesTheModeAndTheScopeInTheSummary(t *testing.T) {
	t.Setenv("CI", "")
	dir := gitModule(t)
	writeIn(t, dir, "leaf/leaf.go", "package leaf\n\nfunc Leaf() int { return 2 }\n")
	rec := &recorder{}
	code, out, errOut := runWith(rec, "--root", dir, "run", "--changed")
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	for _, want := range []string{"mode: changed (files changed against HEAD", "scope: 1 changed file; packages: ./leaf, ./mid"} {
		if !strings.Contains(out, want) {
			t.Errorf("summary lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "./other") {
		t.Errorf("the unrelated package is in scope:\n%s", out)
	}
	if rec.ctx.Diff == nil || !rec.ctx.Diff.Narrow() || !slices.Equal(rec.ctx.Diff.Packages[""], []string{"./leaf", "./mid"}) {
		t.Errorf("context diff %+v", rec.ctx.Diff)
	}
}

func TestRunChangedBaseCoversACommittedChange(t *testing.T) {
	t.Setenv("CI", "")
	dir := gitModule(t)
	gitIn(t, dir, "checkout", "-b", "feature")
	writeIn(t, dir, "other/other.go", "package other\n\nfunc Other() int { return 3 }\n")
	gitIn(t, dir, "commit", "-am", "change other")
	rec := &recorder{}
	code, out, errOut := runWith(rec, "--root", dir, "run", "--changed", "--base", "main")
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	for _, want := range []string{"since the merge base of main and HEAD", "packages: ./other"} {
		if !strings.Contains(out, want) {
			t.Errorf("summary lacks %q:\n%s", want, out)
		}
	}
}

func TestRunChangedGoModChangeWidensAndSaysWhy(t *testing.T) {
	t.Setenv("CI", "")
	dir := gitModule(t)
	writeIn(t, dir, "go.mod", "module example.com/m\n\ngo 1.26\n\n// edited\n")
	rec := &recorder{}
	code, out, errOut := runWith(rec, "--root", dir, "run", "--changed")
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	if !strings.Contains(out, "scope: whole project (widened: go.mod changed)") {
		t.Errorf("summary:\n%s", out)
	}
	if rec.ctx.Diff == nil || rec.ctx.Diff.Narrow() {
		t.Errorf("context diff %+v", rec.ctx.Diff)
	}
}

func TestRunChangedWidensWhenTheExplicitConfigFileChanges(t *testing.T) {
	t.Setenv("CI", "")
	dir := gitModule(t)
	writeIn(t, dir, "ci/custom.toml", "\n")
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-m", "config")
	writeIn(t, dir, "ci/custom.toml", "# edited\n")
	_, out, errOut := runWith(&recorder{}, "--root", dir, "--config", "ci/custom.toml", "run", "--changed")
	if !strings.Contains(out, "widened: ci/custom.toml changed") {
		t.Errorf("summary:\n%s\n%s", out, errOut)
	}
}

func TestRunChangedWithNothingChangedSaysSoAndStillRunsTheRecordCheck(t *testing.T) {
	t.Setenv("CI", "")
	dir := gitModule(t)
	code, out, errOut := run("--root", dir, "run", "--changed", "-t", "record", "-t", "test", "-t", "golangci")
	if code != 0 {
		t.Fatalf("exit %d\n%s\n%s", code, out, errOut)
	}
	for _, want := range []string{"scope: no changed files; only the repo-wide defences run", "record       passed", "test         skipped (no changed Go packages)", "golangci     skipped (no changed Go packages)"} {
		if !strings.Contains(out, want) {
			t.Errorf("summary lacks %q:\n%s", want, out)
		}
	}
}

func TestRunChangedRecordFailureStillFailsTheRun(t *testing.T) {
	t.Setenv("CI", "")
	dir := gitModule(t)
	writeIn(t, dir, "qa/record.toml", "[[exception]]\nrule = \"errcheck\"\npath = \"a.go\"\njustification = \"legacy\"\n")
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-m", "record")
	code, out, _ := run("--root", dir, "run", "--changed", "-t", "record")
	if code != 1 || !strings.Contains(out, "record") || !strings.Contains(out, "FAILED") {
		t.Fatalf("exit %d\n%s", code, out)
	}
}

func TestRunChangedUsageErrors(t *testing.T) {
	dir := gitModule(t)
	cases := map[string][]string{
		"base without changed": {"run", "--base", "main"},
		"changed with paths":   {"run", "--changed", "-p", "leaf"},
		"unknown base":         {"run", "--changed", "--base", "no-such-ref"},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			code, _, errOut := runWith(&recorder{}, append([]string{"--root", dir}, args...)...)
			if code != 2 || errOut == "" {
				t.Fatalf("exit %d, stderr %q", code, errOut)
			}
		})
	}
}

func TestRunHelpDocumentsDiffMode(t *testing.T) {
	_, _, errOut := runWith(&recorder{}, "run", "--help")
	for _, want := range []string{"-changed", "-base"} {
		if !strings.Contains(errOut, want) {
			t.Errorf("help lacks %q:\n%s", want, errOut)
		}
	}
}
