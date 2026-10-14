// Package deferinloop is the bundled analyzer gqaci_defer_in_loop: a defer in a loop body runs when
// the function returns, not when the iteration ends.
package deferinloop

import (
	"go/ast"
	"reflect"

	"golang.org/x/tools/go/analysis"

	"github.com/LongTermSupport/go-qa-ci/analyzers/internal/typesx"
)

// Name is the analyzer name: the identifier every finding is printed under.
const Name = "gqaci_defer_in_loop"

const doc = `gqaci_defer_in_loop forbids defer inside a loop body

Flags a defer statement that is inside a for or range loop of the same function (a function
literal starts a new function, so a defer in a closure called from the loop is not flagged).

A deferred call runs when the function returns. In a loop it piles up one pending call per
iteration: file descriptors, locks and connections stay held until the whole loop is done, and a
long loop can exhaust them. Move the loop body into its own function (or a function literal called
at once) so each iteration returns, or release the resource explicitly at the end of the iteration.`

// Analyzer is the analyzer.
var Analyzer = &analysis.Analyzer{
	Name:       Name,
	Doc:        doc,
	ResultType: reflect.TypeFor[struct{}](),
	Run:        run,
}

func run(pass *analysis.Pass) (any, error) {
	typesx.Inspect(pass, func(node ast.Node, stack []ast.Node) bool {
		if deferred, ok := node.(*ast.DeferStmt); ok && insideLoop(stack) {
			pass.Reportf(deferred.Pos(), "defer inside a loop runs when the function returns, not when the iteration ends; move the loop body into a function")
		}
		return true
	})
	return struct{}{}, nil
}

// insideLoop says whether the nearest enclosing function boundary or loop on the stack is a loop.
func insideLoop(stack []ast.Node) bool {
	for i := len(stack) - 1; i >= 0; i-- {
		switch stack[i].(type) {
		case *ast.FuncDecl, *ast.FuncLit:
			return false
		case *ast.ForStmt, *ast.RangeStmt:
			return true
		}
	}
	return false
}
