// Package exitoutsidemain is the bundled analyzer gqaci_exit_outside_main: the process ends only
// from the program's entry point, never from a function other code calls.
package exitoutsidemain

import (
	"go/ast"
	"go/types"
	"reflect"

	"golang.org/x/tools/go/analysis"

	"github.com/LongTermSupport/go-qa-ci/analyzers/internal/typesx"
)

// Name is the analyzer name: the identifier every finding is printed under.
const Name = "gqaci_exit_outside_main"

const doc = `gqaci_exit_outside_main forbids ending the process outside main

Flags any use (a call, a method value, a function stored in a variable) of os.Exit, syscall.Exit,
log.Fatal, log.Fatalf, log.Fatalln or the same methods of *log.Logger, unless it is lexically inside
the function main of package main, or inside TestMain of a test file.

Exiting skips every deferred call in every goroutine and cannot be recovered or observed by the
caller: files stay unflushed, locks stay held, and the function cannot be tested, because the test
binary dies with it. Return an error to main and let main decide the exit code.`

// Analyzer is the analyzer.
var Analyzer = &analysis.Analyzer{
	Name:       Name,
	Doc:        doc,
	ResultType: reflect.TypeFor[struct{}](),
	Run:        run,
}

func run(pass *analysis.Pass) (any, error) {
	typesx.EachUse(pass, func(id *ast.Ident, obj types.Object, stack []ast.Node) {
		name := exitName(obj)
		if name == "" || allowedPlace(pass, id, stack) {
			return
		}
		pass.Reportf(id.Pos(), "%s ends the process and skips every deferred call; return an error to main instead", name)
	})
	return struct{}{}, nil
}

var fatals = []string{"Fatal", "Fatalf", "Fatalln"}

// exitName names the process-ending function obj is, or returns "".
func exitName(obj types.Object) string {
	switch {
	case typesx.IsPkgFunc(obj, "os", "Exit"):
		return "os.Exit"
	case typesx.IsPkgFunc(obj, "syscall", "Exit"):
		return "syscall.Exit"
	case typesx.IsPkgFunc(obj, "log", fatals...):
		return "log." + obj.Name()
	case typesx.IsMethod(obj, "log", "Logger", fatals...):
		return "log.Logger." + obj.Name()
	}
	return ""
}

// allowedPlace says whether the use is inside func main of package main, or TestMain of a test file.
func allowedPlace(pass *analysis.Pass, id *ast.Ident, stack []ast.Node) bool {
	for _, node := range stack {
		decl, ok := node.(*ast.FuncDecl)
		if !ok || decl.Recv != nil {
			continue
		}
		switch {
		case decl.Name.Name == "main" && pass.Pkg.Name() == "main":
			return true
		case decl.Name.Name == "TestMain" && typesx.IsTestFile(pass, id.Pos()) && takesTestingM(pass, decl):
			return true
		}
	}
	return false
}

func takesTestingM(pass *analysis.Pass, decl *ast.FuncDecl) bool {
	params := decl.Type.Params.List
	return len(params) == 1 && typesx.NamedIs(pass.TypesInfo.TypeOf(params[0].Type), "testing", "M")
}
