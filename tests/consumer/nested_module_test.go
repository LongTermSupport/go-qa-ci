// Package consumer_test drives go-qa end to end over a fixture consuming project whose analyzer
// rules live in a nested Go module, so the project's own go.mod (and so the dependencies of the
// binaries it ships) stays free of golang.org/x/tools and of go-qa-ci itself.
package consumer_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/cli"
)

const rootGoMod = "module example.com/consumer\n\ngo 1.26\n"

const nestedDriver = `package main

import (
	"go/ast"
	"reflect"

	"golang.org/x/tools/go/analysis"

	"github.com/LongTermSupport/go-qa-ci/driver"
)

var noPanic = &analysis.Analyzer{
	Name:       "project_nopanic",
	Doc:        "forbids panic\n\nReturn an error instead of calling panic.",
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

func readIn(t *testing.T, dir, rel string) string {
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
	data, err := handle.ReadFile(rel)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func put(t *testing.T, dir, rel, content string) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// nestedModule requires go-qa-ci from this checkout and carries the dependency set go-qa-ci builds
// with, so the driver builds offline.
func nestedModule(t *testing.T) (goMod, goSum string) {
	t.Helper()
	repo, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	mod := readIn(t, repo, "go.mod")
	sum := readIn(t, repo, "go.sum")
	lines := strings.SplitN(mod, "\n", 2)
	return "module example.com/consumer/qa\n" + lines[1] +
		"\nrequire github.com/LongTermSupport/go-qa-ci v0.0.0\n\nreplace github.com/LongTermSupport/go-qa-ci => " + repo + "\n", sum
}

// consumer is a project whose product module has no dependencies and whose rules are in ./qa.
func consumer(t *testing.T, config string) string {
	t.Helper()
	dir := t.TempDir()
	goMod, goSum := nestedModule(t)
	put(t, dir, "go.mod", rootGoMod)
	put(t, dir, "p.go", "package p\n\nfunc F() { panic(\"x\") }\n")
	put(t, dir, "qa/go.mod", goMod)
	put(t, dir, "qa/go.sum", goSum)
	put(t, dir, "qa/driver/main.go", nestedDriver)
	put(t, dir, "qa/go-qa.toml", config)
	return dir
}

type result struct {
	code     int
	out, err string
}

func run(args ...string) result {
	var out, errOut bytes.Buffer
	code := cli.Run(args, &out, &errOut)
	return result{code, out.String(), errOut.String()}
}

const nestedConfig = "[analyzers]\nmodule = \"qa\"\ndriver = \"./driver\"\n"

func TestARuleDriverInANestedModuleFailsTheAnalyzersStepEndToEnd(t *testing.T) {
	dir := consumer(t, nestedConfig)
	r := run("--root", dir, "run", "-t", "analyzers")
	if r.code != 1 || !strings.Contains(r.err, "error[project_nopanic]") || !strings.Contains(r.err, "--> p.go:3") {
		t.Fatalf("got %+v", r)
	}
	if strings.Contains(r.err, "qa/driver") {
		t.Errorf("the nested module was analysed as part of the product module: %s", r.err)
	}
}

func TestARuleDriverInANestedModuleIsListedAndDocumented(t *testing.T) {
	dir := consumer(t, nestedConfig)
	if r := run("--root", dir, "rules"); r.code != 0 || !strings.Contains(r.out, "project_nopanic") {
		t.Errorf("rules: %+v", r)
	}
	if r := run("--root", dir, "rule-doc", "project_nopanic"); r.code != 0 || !strings.Contains(r.out, "Return an error instead of calling panic") {
		t.Errorf("rule-doc: %+v", r)
	}
}

func TestTheProductModuleStaysDependencyFree(t *testing.T) {
	dir := consumer(t, nestedConfig)
	if r := run("--root", dir, "run", "-t", "analyzers"); r.code != 1 {
		t.Fatalf("got %+v", r)
	}
	if got := readIn(t, dir, "go.mod"); got != rootGoMod {
		t.Errorf("building the nested driver changed the product go.mod:\n%s", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "go.sum")); err == nil {
		t.Error("the product module gained a go.sum")
	}
}

func TestNamingTheDriverWithoutItsModuleFailsLoudly(t *testing.T) {
	dir := consumer(t, "[analyzers]\ndriver = \"./qa/driver\"\n")
	r := run("--root", dir, "run", "-t", "analyzers")
	if r.code == 0 || !strings.Contains(r.err+r.out, "building the analyzer driver") {
		t.Fatalf("a driver outside the root module was accepted: %+v", r)
	}
}
