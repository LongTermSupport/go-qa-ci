// Package unconditionalskip is the bundled analyzer gqaci_unconditional_skip: a test whose body
// skips itself on its first level never runs.
package unconditionalskip

import (
	"go/ast"
	"go/types"
	"reflect"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/LongTermSupport/go-qa-ci/analyzers/internal/typesx"
)

// Name is the analyzer name: the identifier every finding is printed under.
const Name = "gqaci_unconditional_skip"

const doc = `gqaci_unconditional_skip forbids a test that skips itself unconditionally

Flags a call of Skip, Skipf or SkipNow of the testing package that is a statement at the top level
of the body of a Test, Benchmark or Fuzz function, or of a function literal passed to Run, in a
_test.go file. Skips guarded by a condition (an if, a switch) and skips in helper functions are not
flagged.

Everything after such a call is dead, and the test reports as skipped, which a green run hides:
the defect it was written to catch is no longer checked and nobody is told. Delete the test, fix
it, or guard the skip with the condition that makes it legitimate.`

// Analyzer is the analyzer.
var Analyzer = &analysis.Analyzer{
	Name:       Name,
	Doc:        doc,
	ResultType: reflect.TypeFor[struct{}](),
	Run:        run,
}

func run(pass *analysis.Pass) (any, error) {
	typesx.Inspect(pass, func(node ast.Node, _ []ast.Node) bool {
		switch fn := node.(type) {
		case *ast.FuncDecl:
			if fn.Body != nil && fn.Recv == nil && typesx.IsTestFile(pass, fn.Pos()) && isTestName(fn.Name.Name) && takesTesting(pass, fn.Type) {
				check(pass, fn.Body)
			}
		case *ast.CallExpr:
			if lit := runBody(pass, fn); lit != nil {
				check(pass, lit.Body)
			}
		}
		return true
	})
	return struct{}{}, nil
}

func isTestName(name string) bool {
	for _, prefix := range []string{"Test", "Benchmark", "Fuzz"} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

func takesTesting(pass *analysis.Pass, sig *ast.FuncType) bool {
	params := sig.Params.List
	return len(params) == 1 && typesx.NamedIs(pass.TypesInfo.TypeOf(params[0].Type), "testing", "T", "B", "F")
}

// runBody is the function literal a call of (*testing.T).Run or (*testing.B).Run is given.
func runBody(pass *analysis.Pass, call *ast.CallExpr) *ast.FuncLit {
	fn := typesx.Callee(pass, call)
	if fn == nil || !typesx.IsMethod(fn, "testing", "", "Run") || len(call.Args) != 2 {
		return nil
	}
	lit, ok := ast.Unparen(call.Args[1]).(*ast.FuncLit)
	if !ok || !typesx.IsTestFile(pass, lit.Pos()) {
		return nil
	}
	return lit
}

// check reports the skip calls that are statements of the body itself.
func check(pass *analysis.Pass, body *ast.BlockStmt) {
	for _, stmt := range body.List {
		expr, ok := stmt.(*ast.ExprStmt)
		if !ok {
			continue
		}
		call, ok := expr.X.(*ast.CallExpr)
		if !ok {
			continue
		}
		if fn := typesx.Callee(pass, call); isSkip(fn) {
			pass.Reportf(call.Pos(), "unconditional %s: the rest of the test never runs and a green run hides it; guard the skip with its reason or fix the test", describe(call, fn))
		}
	}
}

func isSkip(fn *types.Func) bool {
	return fn != nil && typesx.IsMethod(fn, "testing", "", "Skip", "Skipf", "SkipNow")
}

func describe(call *ast.CallExpr, fn *types.Func) string {
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		if recv, ok := sel.X.(*ast.Ident); ok {
			return recv.Name + "." + fn.Name()
		}
	}
	return fn.Name()
}
