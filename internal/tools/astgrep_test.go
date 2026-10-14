package tools_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/LongTermSupport/go-qa-ci/internal/astgrep"
	"github.com/LongTermSupport/go-qa-ci/internal/astgreptest"
	"github.com/LongTermSupport/go-qa-ci/internal/pipeline"
	"github.com/LongTermSupport/go-qa-ci/internal/tools"
)

const (
	testedRule   = "id: no-println\nlanguage: go\nseverity: error\nmessage: no println\nrule:\n  pattern: println($$$)\n"
	testedCases  = "id: no-println\nvalid: ['fmt.Sprint(1)']\ninvalid: ['println(1)']\n"
	sgconfigBase = "ruleDirs:\n  - rules\ntestConfigs:\n  - testDir: rule-tests\n"
	printlnMatch = `[{"range":{"start":{"line":6,"column":1}},"file":"p/p.go","ruleId":"no-println","severity":"error","message":"no println","note":null}]`
	hintMatch    = `[{"range":{"start":{"line":6,"column":1}},"file":"p/p.go","ruleId":"no-println","severity":"hint","message":"no println","note":null}]`
	findingsOnP  = "[[exception]]\nrule = \"no-println\"\npath = \"p/p.go\"\ncovers = \"findings\"\njustification = \"the fixture prints on purpose\"\n"
	packRule     = "id: gqaci.ast.example\nlanguage: go\nseverity: error\nmessage: example\nnote: \"go-qa rule-doc gqaci.ast.example\"\nrule:\n  pattern: x\n"
)

// projectFiles is a Go module with a tested ast-grep rule.
func projectFiles() map[string]string {
	return map[string]string{
		"p/p.go":                 cleanSource,
		"sgconfig.yml":           sgconfigBase,
		"rules/no-println.yml":   testedRule,
		"rule-tests/println.yml": testedCases,
	}
}

func runAstGrep(t *testing.T, ctx tools.Context, pack astgrep.Pack) (pipeline.Outcome, string) {
	t.Helper()
	ctx.Config = cfgOr(ctx.Config)
	ctx.Today = fixedToday(t)
	var stderr bytes.Buffer
	outcome := tools.Executor{Ctx: ctx, Stderr: &stderr, Pack: &pack}.Run(stepNamed(t, "ast-grep"))
	return outcome, stderr.String()
}

func packOf(t *testing.T, files map[string]string) astgrep.Pack {
	t.Helper()
	mapFS := fstest.MapFS{}
	for name, text := range files {
		mapFS[name] = &fstest.MapFile{Data: []byte(text)}
	}
	pack, err := astgrep.LoadPack(mapFS)
	if err != nil {
		t.Fatal(err)
	}
	return pack
}

func examplePack(t *testing.T) astgrep.Pack {
	t.Helper()
	return packOf(t, map[string]string{"gqaci.ast.example.yml": packRule})
}

func TestAstGrepIsSkippedWithoutAProjectAndWithoutAPack(t *testing.T) {
	astgreptest.Install(t, "[]", 0)
	root := fixture(t, map[string]string{"p/p.go": cleanSource})
	outcome, stderr := runAstGrep(t, tools.Context{Root: root}, astgrep.Pack{})
	if outcome.Kind != pipeline.KindSkipped || !strings.Contains(outcome.Reason, "sgconfig.yml") || stderr != "" {
		t.Errorf("got %+v\n%s", outcome, stderr)
	}
}

func TestAstGrepFailsWhenTheBinaryIsMissing(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	root := fixture(t, projectFiles())
	outcome, stderr := runAstGrep(t, tools.Context{Root: root}, astgrep.Pack{})
	if outcome.Kind != pipeline.KindFailed || !strings.Contains(stderr, "ast-grep is not installed") {
		t.Errorf("got %+v\n%s", outcome, stderr)
	}
}

func TestAstGrepStillJudgesTheRulesWhenTheBinaryIsMissing(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	files := projectFiles()
	files["rules/anon.yml"] = "language: go\nseverity: error\nmessage: m\nrule:\n  pattern: x\n"
	outcome, stderr := runAstGrep(t, tools.Context{Root: fixture(t, files)}, astgrep.Pack{})
	if outcome.Kind != pipeline.KindFailed || !strings.Contains(stderr, "error[gqaci.rules.unnamed]") {
		t.Errorf("got %+v\n%s", outcome, stderr)
	}
}

func TestAstGrepPassesACleanTestedProjectAndRunsTheRuleTests(t *testing.T) {
	argsFile := astgreptest.Install(t, "[]", 0)
	outcome, stderr := runAstGrep(t, tools.Context{Root: fixture(t, projectFiles())}, astgrep.Pack{})
	if outcome.Kind != pipeline.KindPassed || stderr != "" {
		t.Fatalf("got %+v\n%s", outcome, stderr)
	}
	if got := strings.Join(astgreptest.Args(t, argsFile), " "); got != "test --skip-snapshot-tests" {
		t.Errorf("the last invocation was %q", got)
	}
}

func TestAstGrepFailsTheRuleTestsAndPrintsTheirOutput(t *testing.T) {
	astgreptest.Install(t, "[]", 1)
	root := fixture(t, projectFiles())
	outcome, stderr := runAstGrep(t, tools.Context{Root: root}, astgrep.Pack{})
	if outcome.Kind != pipeline.KindFailed || !strings.Contains(stderr, "ast-grep test") {
		t.Errorf("got %+v\n%s", outcome, stderr)
	}
}

func TestAstGrepFailsABlockingMatchWithTheRuleIdentifier(t *testing.T) {
	astgreptest.InstallScanOnly(t, printlnMatch, 1)
	outcome, stderr := runAstGrep(t, tools.Context{Root: fixture(t, projectFiles())}, astgrep.Pack{})
	if outcome.Kind != pipeline.KindFailed {
		t.Fatalf("got %+v\n%s", outcome, stderr)
	}
	if !strings.Contains(stderr, "error[no-println]: no println") || !strings.Contains(stderr, "--> p/p.go:7") {
		t.Errorf("stderr:\n%s", stderr)
	}
}

func TestAstGrepDoesNotBlockOnAMatchBelowErrorSeverity(t *testing.T) {
	astgreptest.Install(t, hintMatch, 0)
	outcome, stderr := runAstGrep(t, tools.Context{Root: fixture(t, projectFiles())}, astgrep.Pack{})
	if outcome.Kind != pipeline.KindPassed || stderr != "" {
		t.Errorf("got %+v\n%s", outcome, stderr)
	}
}

func TestAstGrepAppliesTheRecordToFindings(t *testing.T) {
	astgreptest.InstallScanOnly(t, printlnMatch, 1)
	root := fixture(t, projectFiles())
	writeRecord(t, root, findingsOnP)
	outcome, stderr := runAstGrep(t, tools.Context{Root: root}, astgrep.Pack{})
	if outcome.Kind != pipeline.KindPassed || stderr != "" {
		t.Errorf("got %+v\n%s", outcome, stderr)
	}
}

func TestAstGrepFailsAnUnusedFindingsExceptionOnAFullRunOnly(t *testing.T) {
	astgreptest.Install(t, "[]", 0)
	root := fixture(t, projectFiles())
	writeRecord(t, root, findingsOnP)
	outcome, stderr := runAstGrep(t, tools.Context{Root: root}, astgrep.Pack{})
	if outcome.Kind != pipeline.KindFailed || !strings.Contains(stderr, "error[gqaci.record.stale]") {
		t.Fatalf("full run: %+v\n%s", outcome, stderr)
	}
	outcome, stderr = runAstGrep(t, tools.Context{Root: root, Scope: []string{"p"}}, astgrep.Pack{})
	if outcome.Kind != pipeline.KindPassed || stderr != "" {
		t.Errorf("a scoped run cannot judge staleness: %+v\n%s", outcome, stderr)
	}
}

func TestAstGrepLeavesTheStalenessOfOtherDetectorsExceptionsToThem(t *testing.T) {
	astgreptest.Install(t, "[]", 0)
	root := fixture(t, projectFiles())
	writeRecord(t, root, strings.ReplaceAll(findingsOnP, "no-println", "errcheck"))
	outcome, stderr := runAstGrep(t, tools.Context{Root: root}, astgrep.Pack{})
	if outcome.Kind != pipeline.KindPassed || stderr != "" {
		t.Errorf("got %+v\n%s", outcome, stderr)
	}
}

func TestAstGrepDropsMatchesOutsideTheScopeAndPassesTheScopeOn(t *testing.T) {
	argsFile := astgreptest.InstallScanOnly(t, printlnMatch, 1)
	files := projectFiles()
	delete(files, "rule-tests/println.yml")
	files["sgconfig.yml"] = "ruleDirs:\n  - rules\n"
	root := fixture(t, files)
	outcome, stderr := runAstGrep(t, tools.Context{Root: root, Scope: []string{"q"}}, astgrep.Pack{})
	if outcome.Kind != pipeline.KindFailed || !strings.Contains(stderr, "gqaci.rules.untested") {
		t.Fatalf("got %+v\n%s", outcome, stderr)
	}
	if strings.Contains(stderr, "error[no-println]") {
		t.Errorf("a match outside the scope was reported:\n%s", stderr)
	}
	if got := strings.Join(astgreptest.Args(t, argsFile), " "); got != "scan --json=compact q" {
		t.Errorf("args = %q", got)
	}
}

func TestAstGrepFailsAScanThatDidNotRun(t *testing.T) {
	astgreptest.Install(t, "Error: no such path", 2)
	outcome, stderr := runAstGrep(t, tools.Context{Root: fixture(t, projectFiles())}, astgrep.Pack{})
	if outcome.Kind != pipeline.KindFailed || !strings.Contains(stderr, "ast-grep") {
		t.Errorf("got %+v\n%s", outcome, stderr)
	}
}

func TestAstGrepFailsARuleWithoutAnIdAndARuleWithoutBothKindsOfTestCase(t *testing.T) {
	astgreptest.Install(t, "[]", 0)
	files := projectFiles()
	files["rules/anon.yml"] = "language: go\nseverity: error\nmessage: m\nrule:\n  pattern: x\n"
	files["rules/no-todo.yml"] = "id: no-todo\nlanguage: go\nseverity: error\nmessage: m\nrule:\n  pattern: x\n"
	files["rules/soft.yml"] = "id: soft\nlanguage: go\nseverity: warning\nmessage: m\nrule:\n  pattern: x\n"
	files["rule-tests/no-todo.yml"] = "id: no-todo\nvalid: ['a']\n"
	outcome, stderr := runAstGrep(t, tools.Context{Root: fixture(t, files)}, astgrep.Pack{})
	if outcome.Kind != pipeline.KindFailed {
		t.Fatalf("got %+v\n%s", outcome, stderr)
	}
	for _, want := range []string{
		"error[gqaci.rules.unnamed]", "--> rules/anon.yml",
		"error[gqaci.rules.untested]", "`no-todo`", "--> rules/no-todo.yml",
	} {
		if !strings.Contains(stderr, want) {
			t.Errorf("missing %q in:\n%s", want, stderr)
		}
	}
	if strings.Contains(stderr, "`soft`") || strings.Contains(stderr, "`no-println`") {
		t.Errorf("a warning-level or tested rule was flagged:\n%s", stderr)
	}
}

func TestAstGrepScansWithTheBundledPackFromOutsideTheProjectTree(t *testing.T) {
	argsFile := astgreptest.Install(t, "[]", 0)
	root := fixture(t, map[string]string{"p/p.go": cleanSource})
	outcome, stderr := runAstGrep(t, tools.Context{Root: root, Scope: []string{"p"}}, examplePack(t))
	if outcome.Kind != pipeline.KindPassed || stderr != "" {
		t.Fatalf("got %+v\n%s", outcome, stderr)
	}
	args := astgreptest.Args(t, argsFile)
	if len(args) < 4 || args[0] != "scan" || args[2] != "--rule" || filepath.Base(args[3]) != astgrep.PackFile {
		t.Fatalf("args = %q", args)
	}
	if strings.HasPrefix(args[3], root) {
		t.Errorf("the pack was written inside the project: %s", args[3])
	}
	if args[len(args)-1] != "p" {
		t.Errorf("scope not passed: %q", args)
	}
}

func TestAstGrepExcludesTheConfiguredGlobsFromTheBundledScan(t *testing.T) {
	argsFile := astgreptest.Install(t, "[]", 0)
	root := fixture(t, map[string]string{"p/p.go": cleanSource})
	ctx := tools.Context{Root: root, Config: cfgOr(tools.Context{}.Config)}
	ctx.Config.ScanExclude = []string{"gen/generated.go"}
	if outcome, stderr := runAstGrep(t, ctx, examplePack(t)); outcome.Kind != pipeline.KindPassed {
		t.Fatalf("got %+v\n%s", outcome, stderr)
	}
	if got := strings.Join(astgreptest.Args(t, argsFile), " "); !strings.Contains(got, "--globs !gen/generated.go") {
		t.Errorf("args = %q", got)
	}
}

func TestAstGrepBundledPackCanBeSwitchedOff(t *testing.T) {
	argsFile := astgreptest.Install(t, "[]", 0)
	root := fixture(t, map[string]string{"p/p.go": cleanSource})
	ctx := tools.Context{Root: root, Config: cfgOr(tools.Context{}.Config)}
	ctx.Config.AstGrep.Bundled = false
	outcome, stderr := runAstGrep(t, ctx, examplePack(t))
	if outcome.Kind != pipeline.KindSkipped || stderr != "" {
		t.Errorf("got %+v\n%s", outcome, stderr)
	}
	if _, err := os.Stat(argsFile); err == nil {
		t.Error("ast-grep ran")
	}
}

func TestAstGrepReportsBundledAndProjectFindingsInOneRun(t *testing.T) {
	bundledMatch := strings.ReplaceAll(printlnMatch, "no-println", "gqaci.ast.example")
	astgreptest.InstallScanOnly(t, strings.TrimSuffix(printlnMatch, "]")+","+strings.TrimPrefix(bundledMatch, "["), 1)
	outcome, stderr := runAstGrep(t, tools.Context{Root: fixture(t, projectFiles())}, examplePack(t))
	if outcome.Kind != pipeline.KindFailed {
		t.Fatalf("got %+v\n%s", outcome, stderr)
	}
	if !strings.Contains(stderr, "error[no-println]") || !strings.Contains(stderr, "error[gqaci.ast.example]") {
		t.Errorf("stderr:\n%s", stderr)
	}
}

func TestAstGrepFailsAMalformedSgconfig(t *testing.T) {
	astgreptest.Install(t, "[]", 0)
	files := projectFiles()
	files["sgconfig.yml"] = "ruleDirs: [unterminated\n"
	outcome, stderr := runAstGrep(t, tools.Context{Root: fixture(t, files)}, astgrep.Pack{})
	if outcome.Kind != pipeline.KindFailed || !strings.Contains(stderr, "sgconfig.yml") {
		t.Errorf("got %+v\n%s", outcome, stderr)
	}
}

func TestAstGrepLeavesNothingInTheProjectTree(t *testing.T) {
	astgreptest.Install(t, "[]", 0)
	root := fixture(t, projectFiles())
	before := listFiles(t, root)
	if outcome, stderr := runAstGrep(t, tools.Context{Root: root}, examplePack(t)); outcome.Kind != pipeline.KindPassed {
		t.Fatalf("got %+v\n%s", outcome, stderr)
	}
	if after := listFiles(t, root); strings.Join(before, "\n") != strings.Join(after, "\n") {
		t.Errorf("tree changed:\nbefore %v\nafter %v", before, after)
	}
}

func listFiles(t *testing.T, root string) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(root, func(path string, _ os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}
