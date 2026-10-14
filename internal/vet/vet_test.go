package vet_test

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/finding"
	"github.com/LongTermSupport/go-qa-ci/internal/vet"
	"github.com/LongTermSupport/go-qa-ci/internal/vettest"
)

func TestMain(m *testing.M) {
	vettest.Serve()
	os.Exit(m.Run())
}

func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, text := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func fixtureModule(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	files["go.mod"] = "module example.com/fixture\n\ngo 1.26\n"
	writeFiles(t, dir, files)
	return dir
}

const forbiddenSource = "package p\n\nfunc Forbidden() {}\n\nfunc F() { Forbidden() }\n"

func self(t *testing.T) vet.Tool {
	t.Helper()
	tool, err := vet.Self()
	if err != nil {
		t.Fatal(err)
	}
	return tool
}

func TestParseKeysEveryFindingByItsAnalyzerAndMakesPathsRelative(t *testing.T) {
	dir := t.TempDir()
	data := `{"example.com/p":{"gqaci_test":[{"posn":"` + filepath.Join(dir, "sub", "p.go") + `:5:12","end":"x","message":"call to Forbidden"}],
	"other":[{"posn":"` + filepath.Join(dir, "q.go") + `:1:1","message":"m"}]}}
{"example.com/p [example.com/p.test]":{"gqaci_test":[{"posn":"` + filepath.Join(dir, "sub", "p.go") + `:5:12","message":"call to Forbidden"}]}}`
	got, err := vet.Parse([]byte(data), dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []finding.Finding{
		{Rule: "other", Path: "q.go", Line: 1, Message: "m"},
		{Rule: "gqaci_test", Path: "sub/p.go", Line: 5, Message: "call to Forbidden"},
	}
	if !slices.Equal(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestParseEmptyOutputIsNoFindings(t *testing.T) {
	for _, data := range []string{"", "  \n", "{}", `{"example.com/p":{}}`} {
		got, err := vet.Parse([]byte(data), t.TempDir())
		if err != nil || len(got) != 0 {
			t.Errorf("%q: got %+v, %v", data, got, err)
		}
	}
}

func TestParseRefusesWhatItCannotRead(t *testing.T) {
	dir := t.TempDir()
	inside := filepath.Join(dir, "p.go")
	cases := map[string]string{
		"not json":              "this is not json",
		"analyzer error":        `{"example.com/p":{"gqaci_test":{"error":"it broke"}}}`,
		"position outside":      `{"example.com/p":{"a":[{"posn":"/elsewhere/x.go:1:1","message":"m"}]}}`,
		"position without line": `{"example.com/p":{"a":[{"posn":"` + inside + `","message":"m"}]}}`,
		"position non-numeric":  `{"example.com/p":{"a":[{"posn":"` + inside + `:x:1","message":"m"}]}}`,
		"trailing junk":         `{} junk`,
	}
	for name, data := range cases {
		if got, err := vet.Parse([]byte(data), dir); err == nil {
			t.Errorf("%s: parsed as %+v", name, got)
		}
	}
}

func TestSelfIsTheRunningBinaryAskedToBeTheVetTool(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	tool := self(t)
	if tool.Path != exe || !slices.Contains(tool.Env, vet.EnvVar+"=1") {
		t.Errorf("got %+v", tool)
	}
}

func TestIsVettoolReadsTheMarker(t *testing.T) {
	t.Setenv(vet.EnvVar, "")
	if vet.IsVettool() {
		t.Error("an empty marker counts as set")
	}
	t.Setenv(vet.EnvVar, "1")
	if !vet.IsVettool() {
		t.Error("the marker is ignored")
	}
}

func TestAnalyzersListsEveryRegisteredAnalyzerWithItsSummary(t *testing.T) {
	got, err := self(t).Analyzers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []vet.Analyzer{{Name: "gqaci_test", Summary: "reports calls to a function named Forbidden"}}
	if !slices.Equal(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestHelpPrintsTheAnalyzerDocumentation(t *testing.T) {
	text, err := self(t).Help(context.Background(), "gqaci_test")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "the fix is to call something else") {
		t.Errorf("got %q", text)
	}
	if _, err := self(t).Help(context.Background(), "no_such_analyzer"); err == nil {
		t.Error("an unknown analyzer has help")
	}
}

func TestVetReportsFindingsThoughGoVetItselfExitsZero(t *testing.T) {
	dir := fixtureModule(t, map[string]string{
		"p.go":      forbiddenSource,
		"p_test.go": "package p\n\nimport \"testing\"\n\nfunc TestF(t *testing.T) { Forbidden() }\n",
	})
	got, err := self(t).Vet(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []finding.Finding{
		{Rule: "gqaci_test", Path: "p.go", Line: 5, Message: "call to Forbidden"},
		{Rule: "gqaci_test", Path: "p_test.go", Line: 5, Message: "call to Forbidden"},
	}
	if !slices.Equal(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestVetOfACleanModuleIsEmpty(t *testing.T) {
	dir := fixtureModule(t, map[string]string{"p.go": "package p\n\nfunc F() {}\n"})
	got, err := self(t).Vet(context.Background(), dir)
	if err != nil || len(got) != 0 {
		t.Errorf("got %+v, %v", got, err)
	}
}

func TestVetNeverPassesCodeThatDoesNotCompile(t *testing.T) {
	dir := fixtureModule(t, map[string]string{"p.go": "package p\n\nfunc F() { undefined() }\n"})
	_, err := self(t).Vet(context.Background(), dir)
	if err == nil || !strings.Contains(err.Error(), "undefined") {
		t.Errorf("got %v", err)
	}
}

func TestVetFailsWhenTheToolIsMissing(t *testing.T) {
	dir := fixtureModule(t, map[string]string{"p.go": "package p\n"})
	if _, err := (vet.Tool{Path: filepath.Join(dir, "absent")}).Vet(context.Background(), dir); err == nil {
		t.Error("a missing vet tool passed")
	}
}

const projectDriver = `package main

import (
	"go/ast"
	"reflect"

	"golang.org/x/tools/go/analysis"

	"github.com/LongTermSupport/go-qa-ci/driver"
)

var noPanic = &analysis.Analyzer{
	Name: "project_nopanic",
	Doc:  "forbids panic\n\nCall an error-returning function instead of panic.",
	ResultType: reflect.TypeFor[struct{}](),
	Run: func(pass *analysis.Pass) (any, error) {
		for _, file := range pass.Files {
			ast.Inspect(file, func(n ast.Node) bool {
				if call, ok := n.(*ast.CallExpr); ok {
					if id, isIdent := call.Fun.(*ast.Ident); isIdent && id.Name == "panic" {
						pass.Reportf(call.Pos(), "panic is forbidden")
					}
				}
				return true
			})
		}
		return struct{}{}, nil
	},
}

func main() { driver.Main(noPanic) }
`

// projectModule is a module with a project driver that requires go-qa-ci from this checkout.
func projectModule(t *testing.T) string {
	t.Helper()
	repo, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	checkout, err := os.OpenRoot(repo)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := checkout.Close(); closeErr != nil {
			t.Error(closeErr)
		}
	}()
	goMod, err := checkout.ReadFile("go.mod")
	if err != nil {
		t.Fatal(err)
	}
	goSum, err := checkout.ReadFile("go.sum")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.SplitN(string(goMod), "\n", 2)
	module := "module example.com/project\n" + lines[1] +
		"\nrequire github.com/LongTermSupport/go-qa-ci v0.0.0\n\nreplace github.com/LongTermSupport/go-qa-ci => " + repo + "\n"
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"go.mod":                     module,
		"go.sum":                     string(goSum),
		"internal/qa/driver/main.go": projectDriver,
		"p.go":                       "package p\n\nfunc F() { panic(\"x\") }\n",
	})
	return dir
}

func TestBuildProducesAVetToolFromTheProjectDriver(t *testing.T) {
	dir := projectModule(t)
	tool, cleanup, err := vet.Build(context.Background(), dir, "./internal/qa/driver")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	listed, err := tool.Analyzers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if want := []vet.Analyzer{{Name: "project_nopanic", Summary: "forbids panic"}}; !slices.Equal(listed, want) {
		t.Errorf("got %+v, want %+v", listed, want)
	}
	found, err := tool.Vet(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if want := []finding.Finding{{Rule: "project_nopanic", Path: "p.go", Line: 3, Message: "panic is forbidden"}}; !slices.Equal(found, want) {
		t.Errorf("got %+v, want %+v", found, want)
	}
	cleanup()
	if _, err := os.Stat(tool.Path); err == nil {
		t.Error("cleanup left the built driver behind")
	}
}

func TestBuildFailsLoudlyOnABrokenDriver(t *testing.T) {
	dir := projectModule(t)
	if _, _, err := vet.Build(context.Background(), dir, "./internal/qa/missing"); err == nil {
		t.Error("a driver that does not build was accepted")
	}
}

func TestResolveWithoutADriverIsSelfAndWithOneBuildsIt(t *testing.T) {
	tool, cleanup, err := vet.Resolve(context.Background(), t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	cleanup()
	if want := self(t); tool.Path != want.Path || !slices.Equal(tool.Env, want.Env) {
		t.Errorf("got %+v, want %+v", tool, want)
	}
	dir := projectModule(t)
	built, cleanup, err := vet.Resolve(context.Background(), dir, "./internal/qa/driver")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if built.Path == tool.Path {
		t.Error("the project driver was not built")
	}
}
