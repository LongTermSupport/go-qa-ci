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

func TestRunTellsTheExecutorWhichToolsWereSelected(t *testing.T) {
	t.Setenv("CI", "")
	rec := &recorder{}
	code, _, errOut := runWith(rec, "--root", gitModule(t), "run", "-t", "test", "-t", "coverage")
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	if !slices.Contains(rec.ctx.Selected, "test") || !slices.Contains(rec.ctx.Selected, "coverage") {
		t.Errorf("selected %v", rec.ctx.Selected)
	}
}

func TestRunChangedRefusesABaseThatLooksLikeAnOption(t *testing.T) {
	t.Setenv("CI", "")
	dir := gitModule(t)
	code, _, errOut := runWith(&recorder{}, "--root", dir, "run", "--changed", "--base", "--all")
	if code != 2 || !strings.Contains(errOut, "not an option") {
		t.Errorf("exit %d, stderr %q", code, errOut)
	}
}

func TestRunChangedWidensWhenTheExplicitConfigFileChanges(t *testing.T) {
	for name, spelling := range map[string]func(dir string) string{
		"relative":          func(string) string { return "ci/custom.toml" },
		"dot-relative":      func(string) string { return "./ci/custom.toml" },
		"absolute":          func(dir string) string { return filepath.Join(dir, "ci", "custom.toml") },
		"with a parent hop": func(string) string { return "ci/../ci/custom.toml" },
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("CI", "")
			dir := gitModule(t)
			writeIn(t, dir, "ci/custom.toml", "\n")
			gitIn(t, dir, "add", "-A")
			gitIn(t, dir, "commit", "-m", "config")
			writeIn(t, dir, "ci/custom.toml", "# edited\n")
			_, out, errOut := runWith(&recorder{}, "--root", dir, "--config", spelling(dir), "run", "--changed")
			if !strings.Contains(out, "widened: ci/custom.toml changed") {
				t.Errorf("summary:\n%s\n%s", out, errOut)
			}
		})
	}
}

func TestRunChangedWidensOnTheRuleSourcesTheConfigurationNames(t *testing.T) {
	cases := []struct {
		name, config, changed, want string
		extra                       map[string]string
	}{
		{"sgconfig.yml", "", "sgconfig.yml", "widened: sgconfig.yml changed", map[string]string{"sgconfig.yml": "ruleDirs: [rules]\n"}},
		{"an ast-grep rule directory", "", "rules/r.yml", "widened: rules/r.yml changed", map[string]string{"sgconfig.yml": "ruleDirs: [rules]\n", "rules/r.yml": "id: r\n"}},
		{"an ast-grep test directory", "", "rule-tests/r-test.yml", "widened: rule-tests/r-test.yml changed", map[string]string{"sgconfig.yml": "ruleDirs: [rules]\ntestConfigs:\n  - testDir: rule-tests\n", "rule-tests/r-test.yml": "id: r\n"}},
		{"the documentation directory", "docs_dir = \"pages\"\n", "pages/x.md", "widened: pages/x.md changed", map[string]string{"pages/x.md": "# x\n"}},
		{"the analyzer module", "[analyzers]\nmodule = \"tools/rules\"\ndriver = \"./driver\"\n", "tools/rules/notes.txt", "widened: tools/rules/notes.txt changed", map[string]string{"tools/rules/go.mod": "module example.com/rules\n", "tools/rules/r.go": "package rules\n"}},
		{"the driver package of the root module", "[analyzers]\ndriver = \"./internal/qa/driver/...\"\n", "internal/qa/driver/notes.txt", "widened: internal/qa/driver/notes.txt changed", map[string]string{"internal/qa/driver/main.go": "package main\n"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("CI", "")
			dir := module(t, tc.config)
			for rel, content := range tc.extra {
				writeIn(t, dir, rel, content)
			}
			writeIn(t, dir, "other/other.go", "package other\n")
			gitIn(t, dir, "init")
			gitIn(t, dir, "add", "-A")
			gitIn(t, dir, "commit", "-m", "initial")
			edited := "changed\n"
			if tc.changed == "sgconfig.yml" {
				edited = tc.extra["sgconfig.yml"] + "# edited\n"
			}
			writeIn(t, dir, tc.changed, edited)
			_, out, errOut := runWith(&recorder{}, "--root", dir, "run", "--changed")
			if !strings.Contains(out, tc.want) {
				t.Errorf("summary lacks %q:\n%s\n%s", tc.want, out, errOut)
			}
		})
	}
}

func TestRunChangedDoesNotWidenOnADirectoryNamedQaAlone(t *testing.T) {
	t.Setenv("CI", "")
	dir := module(t, "")
	writeIn(t, dir, "qa/notes.md", "n\n")
	writeIn(t, dir, "other/other.go", "package other\n")
	gitIn(t, dir, "init")
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-m", "initial")
	writeIn(t, dir, "qa/notes.md", "changed\n")
	_, out, errOut := runWith(&recorder{}, "--root", dir, "run", "--changed")
	if strings.Contains(out, "widened") {
		t.Errorf("a file under qa/ that nothing configured widened the run:\n%s\n%s", out, errOut)
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
