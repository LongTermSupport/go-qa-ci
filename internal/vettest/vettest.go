// Package vettest is a trivial analyzer and the hook that lets a test binary act as a go vet tool,
// so the analyzers detector is tested against the real go vet protocol without building a driver.
package vettest

import (
	"go/ast"
	"reflect"

	"golang.org/x/tools/go/analysis"

	"github.com/LongTermSupport/go-qa-ci/driver"
	"github.com/LongTermSupport/go-qa-ci/internal/vet"
)

// Analyzer reports every call of a function named Forbidden.
var Analyzer = &analysis.Analyzer{
	Name: "gqaci_test",
	Doc:  "reports calls to a function named Forbidden\n\nA fixture analyzer: calling Forbidden is the defect, so the fix is to call something else.",
	Run:  run,
	// A result type is declared so that run need not return a nil result with a nil error.
	ResultType: reflect.TypeFor[struct{}](),
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			if call, ok := node.(*ast.CallExpr); ok {
				if name, isIdent := call.Fun.(*ast.Ident); isIdent && name.Name == "Forbidden" {
					pass.Reportf(call.Pos(), "call to Forbidden")
				}
			}
			return true
		})
	}
	return struct{}{}, nil
}

// Serve is called first by a test binary's TestMain: when go-qa asked the binary to act as the go
// vet tool it runs Analyzer as that tool and does not return.
func Serve() {
	if vet.IsVettool() {
		driver.Main(Analyzer)
	}
}
