// Package astgrep_test runs the ast-grep detector against the real ast-grep binary: each rule fires
// on a violating Go snippet and stays quiet on a clean one, the project's rule tests pass, an
// ignore comment silences a rule, and a project rule and a bundled rule are reported in one run.
// Every test here needs the binary and is skipped, saying so, when it is not on PATH; the
// detector's own logic (argument vector, exit codes, record, scope) is tested without it in
// internal/tools.
package astgrep_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/LongTermSupport/go-qa-ci/internal/astgrep"
	"github.com/LongTermSupport/go-qa-ci/internal/config"
	"github.com/LongTermSupport/go-qa-ci/internal/date"
	"github.com/LongTermSupport/go-qa-ci/internal/pipeline"
	"github.com/LongTermSupport/go-qa-ci/internal/tools"
)

// requireAstGrep skips the test when the ast-grep binary is not installed.
func requireAstGrep(t *testing.T) astgrep.Runner {
	t.Helper()
	runner, err := astgrep.Locate()
	if err != nil {
		t.Skipf("needs the ast-grep binary: %v", err)
	}
	return runner
}

const (
	printlnRule = "id: no-fmt-println\nlanguage: go\nseverity: error\nmessage: print through the logger\nrule:\n  pattern:\n    context: \"func t() { fmt.Println($$$ARGS) }\"\n    selector: call_expression\n"
	printlnCase = "id: no-fmt-println\nvalid:\n  - |\n    package p\n    func f() { _ = 1 }\ninvalid:\n  - |\n    package p\n    func f() { fmt.Println(1) }\n"
	tempRule    = "id: gqaci.ast.hardcoded-temp-path\nlanguage: go\nseverity: error\nmessage: use os.TempDir or t.TempDir\n" +
		"note: \"go-qa rule-doc gqaci.ast.hardcoded-temp-path\"\nrule:\n  kind: interpreted_string_literal\n  regex: ^\"/(var/)?tmp(/[^\"]*)?\"$\n"
)

// Red-proof fixtures: each is a whole Go file.
var (
	printlnViolating = []string{
		"package p\n\nimport \"fmt\"\n\nfunc f() { fmt.Println(\"x\") }\n",
		"package p\n\nimport \"fmt\"\n\nfunc f(a, b int) { fmt.Println(a, b) }\n",
	}
	printlnClean = []string{
		"package p\n\nimport \"fmt\"\n\nfunc f() string { return fmt.Sprint(1) }\n",
		"package p\n\n// fmt.Println is only mentioned here.\nfunc f() {}\n",
	}
	tempViolating = []string{
		"package p\n\nimport \"os\"\n\nfunc f() error { return os.WriteFile(\"/tmp/report.txt\", nil, 0o600) }\n",
		"package p\n\nconst scratch = \"/var/tmp/scratch\"\n",
	}
	tempClean = []string{
		"package p\n\nimport \"os\"\n\nfunc f() string { return os.TempDir() }\n",
		"package p\n\nvar names = []string{\"/tmpfs\", \"tmp/relative\", \"/srv/tmp\"}\n",
	}
)

// project writes a Go module holding a tested ast-grep project rule and returns its root.
func project(t *testing.T, extra map[string]string) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"go.mod":                 "module example.com/fixture\n\ngo 1.26\n",
		"sgconfig.yml":           "ruleDirs:\n  - rules\ntestConfigs:\n  - testDir: rule-tests\n",
		"rules/no-println.yml":   printlnRule,
		"rule-tests/println.yml": printlnCase,
	}
	for name, text := range extra {
		files[name] = text
	}
	for name, text := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func bundledTempPack(t *testing.T) astgrep.Pack {
	t.Helper()
	pack, err := astgrep.LoadPack(fstest.MapFS{"gqaci.ast.hardcoded-temp-path.yml": {Data: []byte(tempRule)}})
	if err != nil {
		t.Fatal(err)
	}
	return pack
}

func detect(t *testing.T, root string, cfg config.Config, pack astgrep.Pack) (pipeline.Outcome, string) {
	t.Helper()
	today, err := date.Parse("2026-10-07")
	if err != nil {
		t.Fatal(err)
	}
	tool, found := tools.FromName("ast-grep")
	if !found {
		t.Fatal("no ast-grep tool")
	}
	var stderr bytes.Buffer
	executor := tools.Executor{Ctx: tools.Context{Root: root, Config: cfg, Today: today}, Stderr: &stderr, Pack: &pack}
	return executor.Run(tool.Step(0)), stderr.String()
}

// scanIDs are the rule identifiers ast-grep reports for one Go file, scanned with rule only.
func scanIDs(t *testing.T, runner astgrep.Runner, ruleYAML, source string) []string {
	t.Helper()
	dir := t.TempDir()
	rule := filepath.Join(dir, "rule.yml")
	for name, text := range map[string]string{"rule.yml": ruleYAML, "snippet.go": source} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	matches, err := runner.Scan(t.Context(), dir, astgrep.ScanOptions{Rule: rule, Paths: []string{"snippet.go"}})
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]string, len(matches))
	for i, match := range matches {
		ids[i] = match.Rule
	}
	sort.Strings(ids)
	return ids
}

func TestEveryViolatingSnippetFiresItsRuleAndNoOther(t *testing.T) {
	runner := requireAstGrep(t)
	for _, tc := range []struct {
		id        string
		rule      string
		violating []string
	}{
		{"no-fmt-println", printlnRule, printlnViolating},
		{"gqaci.ast.hardcoded-temp-path", tempRule, tempViolating},
	} {
		for _, source := range tc.violating {
			ids := scanIDs(t, runner, tc.rule, source)
			if len(ids) == 0 || ids[0] != tc.id || ids[len(ids)-1] != tc.id {
				t.Errorf("%s: got %v on:\n%s", tc.id, ids, source)
			}
		}
	}
}

func TestEveryCleanSnippetIsQuiet(t *testing.T) {
	runner := requireAstGrep(t)
	for _, tc := range []struct {
		id    string
		rule  string
		clean []string
	}{
		{"no-fmt-println", printlnRule, printlnClean},
		{"gqaci.ast.hardcoded-temp-path", tempRule, tempClean},
	} {
		for _, source := range tc.clean {
			if ids := scanIDs(t, runner, tc.rule, source); len(ids) != 0 {
				t.Errorf("%s fired on a clean snippet: %v\n%s", tc.id, ids, source)
			}
		}
	}
}

func TestAnIgnoreCommentSilencesARule(t *testing.T) {
	runner := requireAstGrep(t)
	const call = "package p\n\nimport \"fmt\"\n\nfunc f() {\n%s\tfmt.Println(1)\n}\n"
	if ids := scanIDs(t, runner, printlnRule, fmt.Sprintf(call, "")); len(ids) != 1 {
		t.Fatalf("positive control: the call without the comment must fire once, got %v", ids)
	}
	source := fmt.Sprintf(call, "\t// ast-grep-ignore: no-fmt-println\n")
	if ids := scanIDs(t, runner, printlnRule, source); len(ids) != 0 {
		t.Errorf("the ignore comment did not silence the rule: %v", ids)
	}
}

func TestTheProjectRuleTestsPassThroughTheRunner(t *testing.T) {
	runner := requireAstGrep(t)
	output, passed, err := runner.Test(t.Context(), project(t, nil))
	if err != nil || !passed {
		t.Errorf("passed=%v err=%v\n%s", passed, err, output)
	}
}

func TestTheDetectorPassesACleanProject(t *testing.T) {
	requireAstGrep(t)
	root := project(t, map[string]string{"p/p.go": printlnClean[0]})
	outcome, stderr := detect(t, root, config.Default(), astgrep.Pack{})
	if outcome.Kind != pipeline.KindPassed || stderr != "" {
		t.Errorf("got %+v\n%s", outcome, stderr)
	}
}

func TestTheDetectorReportsProjectAndBundledFindingsInOneRun(t *testing.T) {
	requireAstGrep(t)
	root := project(t, map[string]string{
		"p/p.go": "package p\n\nimport \"fmt\"\n\nfunc f() { fmt.Println(\"/tmp/x\") }\n\nconst scratch = \"/tmp/scratch\"\n",
	})
	outcome, stderr := detect(t, root, config.Default(), bundledTempPack(t))
	if outcome.Kind != pipeline.KindFailed {
		t.Fatalf("got %+v\n%s", outcome, stderr)
	}
	for _, want := range []string{"error[no-fmt-println]", "error[gqaci.ast.hardcoded-temp-path]", "--> p/p.go:"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("missing %q in:\n%s", want, stderr)
		}
	}
}

func TestTheBundledPackCanBeSwitchedOffAndTheProjectRulesStillRun(t *testing.T) {
	requireAstGrep(t)
	root := project(t, map[string]string{
		"p/p.go": "package p\n\nimport \"fmt\"\n\nfunc f() { fmt.Println(1) }\n\nconst scratch = \"/tmp/scratch\"\n",
	})
	cfg := config.Default()
	cfg.AstGrep.Bundled = false
	outcome, stderr := detect(t, root, cfg, bundledTempPack(t))
	if outcome.Kind != pipeline.KindFailed || !strings.Contains(stderr, "error[no-fmt-println]") || strings.Contains(stderr, "gqaci.ast.") {
		t.Errorf("got %+v\n%s", outcome, stderr)
	}
	if err := os.Remove(filepath.Join(root, "sgconfig.yml")); err != nil {
		t.Fatal(err)
	}
	outcome, stderr = detect(t, root, cfg, bundledTempPack(t))
	if outcome.Kind != pipeline.KindSkipped || !strings.Contains(outcome.Reason, "sgconfig.yml") {
		t.Errorf("without a project and without the pack: %+v\n%s", outcome, stderr)
	}
}

func TestTheDetectorFailsARuleTestThatDoesNotHold(t *testing.T) {
	requireAstGrep(t)
	root := project(t, map[string]string{
		"rule-tests/println.yml": strings.ReplaceAll(printlnCase, "fmt.Println(1)", "_ = 2"),
	})
	outcome, stderr := detect(t, root, config.Default(), astgrep.Pack{})
	if outcome.Kind != pipeline.KindFailed || !strings.Contains(stderr, "ast-grep test") {
		t.Errorf("got %+v\n%s", outcome, stderr)
	}
}

func TestTheDetectorReportsItselfSkippedWithTheReasonWithoutTheBinary(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if _, err := exec.LookPath(astgrep.Binary); err == nil {
		t.Fatal("ast-grep is still on PATH")
	}
	outcome, stderr := detect(t, project(t, nil), config.Default(), astgrep.Pack{})
	if outcome.Kind != pipeline.KindSkipped || !strings.Contains(outcome.Reason, "ast-grep is not installed") {
		t.Errorf("got %+v\n%s", outcome, stderr)
	}
}
