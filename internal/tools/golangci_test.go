package tools_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/config"
	"github.com/LongTermSupport/go-qa-ci/internal/pipeline"
	"github.com/LongTermSupport/go-qa-ci/internal/tools"
)

const (
	errcheckSource  = "// Package p is a fixture.\npackage p\n\nimport \"os\"\n\n// Clean removes a file.\nfunc Clean() {\n\tos.Remove(\"x\")\n}\n"
	deprecatedUse   = "// Package q is a fixture.\npackage q\n\nimport \"io/ioutil\"\n\n// Read reads.\nfunc Read() ([]byte, error) { return ioutil.ReadFile(\"x\") }\n"
	unformattedCode = "// Package p is a fixture.\npackage p\n\n// F is a fixture.\nfunc F() {\n\tprintln( \"hi\")\n}\n"
)

// fixture writes a module under a temporary directory and returns its root.
func fixture(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	files["go.mod"] = "module example.com/fixture\n\ngo 1.26\n"
	for name, text := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func runStep(t *testing.T, ctx tools.Context, name string) (pipeline.Outcome, string) {
	t.Helper()
	ctx.Config = cfgOr(ctx.Config)
	ctx.Today = fixedToday(t)
	var stderr bytes.Buffer
	outcome := tools.Executor{Ctx: ctx, Stderr: &stderr}.Run(stepNamed(t, name))
	return outcome, stderr.String()
}

func cfgOr(c config.Config) config.Config {
	if c.DocsDir == "" {
		return config.Default()
	}
	return c
}

func TestGolangciFailsAnErrcheckViolationWithTheLinterNameAsIdentifier(t *testing.T) {
	root := fixture(t, map[string]string{"p/p.go": errcheckSource})
	outcome, stderr := runStep(t, tools.Context{Root: root}, "golangci")
	if outcome.Kind != pipeline.KindFailed {
		t.Fatalf("got %+v", outcome)
	}
	if !strings.Contains(stderr, "error[errcheck]: ") || !strings.Contains(stderr, "--> p/p.go:8") {
		t.Errorf("stderr:\n%s", stderr)
	}
}

func TestGolangciPrintsStaticcheckCodes(t *testing.T) {
	root := fixture(t, map[string]string{"q/q.go": deprecatedUse})
	outcome, stderr := runStep(t, tools.Context{Root: root}, "golangci")
	if outcome.Kind != pipeline.KindFailed || !strings.Contains(stderr, "error[staticcheck::SA1019]") {
		t.Errorf("got %+v\n%s", outcome, stderr)
	}
}

func TestGolangciPassesACleanModule(t *testing.T) {
	root := fixture(t, map[string]string{"c/c.go": "// Package c is a fixture.\npackage c\n"})
	outcome, stderr := runStep(t, tools.Context{Root: root}, "golangci")
	if outcome.Kind != pipeline.KindPassed || stderr != "" {
		t.Errorf("got %+v\n%s", outcome, stderr)
	}
}

func TestGolangciDropsFindingsOutsideTheScope(t *testing.T) {
	root := fixture(t, map[string]string{"p/p.go": errcheckSource, "q/q.go": deprecatedUse})
	outcome, stderr := runStep(t, tools.Context{Root: root, Scope: []string{"q"}}, "golangci")
	if outcome.Kind != pipeline.KindFailed || !strings.Contains(stderr, "q/q.go") {
		t.Fatalf("scoped finding missing: %+v\n%s", outcome, stderr)
	}
	if strings.Contains(stderr, "p/p.go") || strings.Contains(stderr, "errcheck") {
		t.Errorf("a finding outside the scope was reported:\n%s", stderr)
	}
	outcome, stderr = runStep(t, tools.Context{Root: root, Scope: []string{"p/p.go"}}, "golangci")
	if outcome.Kind != pipeline.KindFailed || strings.Contains(stderr, "q/q.go") {
		t.Errorf("file scope: %+v\n%s", outcome, stderr)
	}
}

func TestGolangciPassesWhenEveryFindingIsOutsideTheScope(t *testing.T) {
	root := fixture(t, map[string]string{"p/p.go": errcheckSource, "c/c.go": "// Package c is a fixture.\npackage c\n"})
	outcome, stderr := runStep(t, tools.Context{Root: root, Scope: []string{"c"}}, "golangci")
	if outcome.Kind != pipeline.KindPassed || stderr != "" {
		t.Errorf("got %+v\n%s", outcome, stderr)
	}
}

func writeRecord(t *testing.T, root, text string) {
	t.Helper()
	path := filepath.Join(root, "qa", "record.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestGolangciAppliesTheRecordToFindingsOnly(t *testing.T) {
	root := fixture(t, map[string]string{"p/p.go": errcheckSource})
	writeRecord(t, root, "[[exception]]\nrule = \"errcheck\"\npath = \"p/p.go\"\ncovers = \"findings\"\njustification = \"fixture\"\n\n"+
		"[[exception]]\nrule = \"gosec::G104\"\npath = \"p/p.go\"\ncovers = \"findings\"\njustification = \"fixture\"\n")
	outcome, stderr := runStep(t, tools.Context{Root: root}, "golangci")
	if strings.Contains(stderr, "error[errcheck]") {
		t.Errorf("a recorded finding was reported:\n%s", stderr)
	}
	if outcome.Kind != pipeline.KindPassed {
		t.Errorf("got %+v\n%s", outcome, stderr)
	}

	writeRecord(t, root, "[[exception]]\nrule = \"errcheck\"\npath = \"p/p.go\"\ncovers = \"suppressions\"\njustification = \"fixture\"\n")
	outcome, stderr = runStep(t, tools.Context{Root: root}, "golangci")
	if outcome.Kind != pipeline.KindFailed || !strings.Contains(stderr, "error[errcheck]") {
		t.Errorf("a suppression exception licensed a finding: %+v\n%s", outcome, stderr)
	}
}

func TestGolangciChecksFurtherModulesAndPrefixesTheirPaths(t *testing.T) {
	root := fixture(t, map[string]string{"c/c.go": "// Package c is a fixture.\npackage c\n"})
	nested := filepath.Join(root, "tools", "x")
	if err := os.MkdirAll(nested, 0o750); err != nil {
		t.Fatal(err)
	}
	for name, text := range map[string]string{"go.mod": "module example.com/x\n\ngo 1.26\n", "x.go": errcheckSource} {
		if err := os.WriteFile(filepath.Join(nested, name), []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cfg := config.Default()
	cfg.Modules = []string{"tools/x"}
	outcome, stderr := runStep(t, tools.Context{Root: root, Config: cfg}, "golangci")
	if outcome.Kind != pipeline.KindFailed || !strings.Contains(stderr, "--> tools/x/x.go:8") {
		t.Errorf("got %+v\n%s", outcome, stderr)
	}
}

func TestGolangciFailsLoudlyWhenTheToolIsMissing(t *testing.T) {
	root := fixture(t, map[string]string{"c/c.go": "// Package c is a fixture.\npackage c\n"})
	t.Setenv("PATH", t.TempDir())
	outcome, stderr := runStep(t, tools.Context{Root: root}, "golangci")
	if outcome.Kind != pipeline.KindFailed || !strings.Contains(stderr, "golangci-lint not found") {
		t.Errorf("got %+v\n%s", outcome, stderr)
	}
}

func TestFmtFixRewritesAndPasses(t *testing.T) {
	root := fixture(t, map[string]string{"p/p.go": unformattedCode})
	outcome, stderr := runStep(t, tools.Context{Root: root, Fix: true}, "fmt")
	if outcome.Kind != pipeline.KindPassed {
		t.Fatalf("got %+v\n%s", outcome, stderr)
	}
	if got := readFixture(t, root, "p/p.go"); strings.Contains(got, "( \"hi\"") {
		t.Errorf("file not formatted: %q", got)
	}
}

func TestFmtCheckFailsWithTheDiffAndLeavesTheFile(t *testing.T) {
	root := fixture(t, map[string]string{"p/p.go": unformattedCode})
	outcome, stderr := runStep(t, tools.Context{Root: root}, "fmt")
	if outcome.Kind != pipeline.KindFailed || !strings.Contains(stderr, "println(\"hi\")") {
		t.Errorf("got %+v\n%s", outcome, stderr)
	}
	if got := readFixture(t, root, "p/p.go"); got != unformattedCode {
		t.Errorf("check mode modified the file: %q", got)
	}
	if strings.Contains(stderr, "writing the diff") || strings.Contains(stderr, "%!") {
		t.Errorf("a spurious error line is printed:\n%s", stderr)
	}
}

func TestFmtCheckReportsEveryModuleWithADiff(t *testing.T) {
	root := fixture(t, map[string]string{"p/p.go": unformattedCode})
	nested := filepath.Join(root, "tools", "x")
	if err := os.MkdirAll(nested, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "go.mod"), []byte("module example.com/nested\n\ngo 1.26\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "n.go"), []byte(strings.Replace(unformattedCode, "package p", "package n", 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Modules = []string{"tools/x"}
	outcome, stderr := runStep(t, tools.Context{Root: root, Config: cfg}, "fmt")
	if outcome.Kind != pipeline.KindFailed || strings.Count(stderr, "error[fmt]") != 2 {
		t.Errorf("both modules must be reported: %+v\n%s", outcome, stderr)
	}
}

func TestGolangciIgnoresTheConsumerConfigsIssueCaps(t *testing.T) {
	const removal = "\tos.Remove(\"x\")\n"
	source := func(name string, calls int) string {
		return "// Package " + name + " is a fixture.\npackage " + name + "\n\nimport \"os\"\n\n// F removes.\nfunc F() {\n" + strings.Repeat(removal, calls) + "}\n"
	}
	root := fixture(t, map[string]string{
		".golangci.yml": "version: \"2\"\nlinters:\n  default: none\n  enable:\n    - errcheck\n",
		"a/a.go":        source("a", 3),
		"z/z.go":        source("z", 1),
	})
	for range 4 {
		outcome, stderr := runStep(t, tools.Context{Root: root, Scope: []string{"z"}}, "golangci")
		if outcome.Kind != pipeline.KindFailed || strings.Count(stderr, "error[errcheck]") != 1 || !strings.Contains(stderr, "z/z.go") {
			t.Fatalf("a capped finding hid the in-scope one: %+v\n%s", outcome, stderr)
		}
	}
	outcome, stderr := runStep(t, tools.Context{Root: root, Scope: []string{"a"}}, "golangci")
	if outcome.Kind != pipeline.KindFailed || strings.Count(stderr, "error[errcheck]") != 3 {
		t.Errorf("identical findings were collapsed: %+v\n%s", outcome, stderr)
	}
}

func TestFmtInCIFailsBecauseTheWorkingTreeChanged(t *testing.T) {
	root := fixture(t, map[string]string{"p/p.go": unformattedCode, "p/ok.go": "// Package p is a fixture.\npackage p\n"})
	outcome, stderr := runStep(t, tools.Context{Root: root, Fix: true, CI: true}, "fmt")
	if outcome.Kind != pipeline.KindFailed || !strings.Contains(stderr, "working tree changed") || !strings.Contains(stderr, "p/p.go") {
		t.Fatalf("got %+v\n%s", outcome, stderr)
	}
	if strings.Contains(stderr, "p/ok.go") {
		t.Errorf("an untouched file is listed:\n%s", stderr)
	}
	if got := readFixture(t, root, "p/p.go"); strings.Contains(got, "( \"hi\"") {
		t.Errorf("CI mode must autofix before judging: %q", got)
	}
	outcome, stderr = runStep(t, tools.Context{Root: root, Fix: true, CI: true}, "fmt")
	if outcome.Kind != pipeline.KindPassed {
		t.Errorf("formatted tree still fails: %+v\n%s", outcome, stderr)
	}
}

func TestFmtRespectsTheScope(t *testing.T) {
	root := fixture(t, map[string]string{"p/p.go": unformattedCode, "q/q.go": unformattedCode})
	outcome, stderr := runStep(t, tools.Context{Root: root, Fix: true, Scope: []string{"q"}}, "fmt")
	if outcome.Kind != pipeline.KindPassed {
		t.Fatalf("got %+v\n%s", outcome, stderr)
	}
	if readFixture(t, root, "p/p.go") != unformattedCode {
		t.Error("a file outside the scope was formatted")
	}
	if got := readFixture(t, root, "q/q.go"); strings.Contains(got, "( \"hi\"") {
		t.Errorf("a file in scope was not formatted: %q", got)
	}
}

func readFixture(t *testing.T, root, name string) string {
	t.Helper()
	handle, err := os.OpenRoot(root)
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
