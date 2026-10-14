// Package testclock is the bundled analyzer gqaci_test_clock: a test must not read the wall clock.
package testclock

import (
	"go/ast"
	"go/types"
	"reflect"

	"golang.org/x/tools/go/analysis"

	"github.com/LongTermSupport/go-qa-ci/analyzers/internal/typesx"
)

// Name is the analyzer name: the identifier every finding is printed under.
const Name = "gqaci_test_clock"

const doc = `gqaci_test_clock forbids reading the wall clock in a test file

Flags any use (a call, a function stored in a variable) of time.Now, time.Since or time.Until in a
_test.go file. Production files are not judged.

A test that depends on the real time is flaky by construction: it passes on a quiet machine and
fails under load, across a midnight, a daylight-saving change or a leap second, and it cannot
reproduce the failure it once had. Inject the clock (a func() time.Time field or parameter) and
give the test a fixed instant.

A test that must read the clock (a timing measurement, a deadline against a real process) is
licensed by a project-record entry with covers = "findings" for this rule and the path of that
file; there is no inline route.`

// Analyzer is the analyzer.
var Analyzer = &analysis.Analyzer{
	Name:       Name,
	Doc:        doc,
	ResultType: reflect.TypeFor[struct{}](),
	Run:        run,
}

func run(pass *analysis.Pass) (any, error) {
	typesx.EachUse(pass, func(id *ast.Ident, obj types.Object, _ []ast.Node) {
		if typesx.IsPkgFunc(obj, "time", "Now", "Since", "Until") && typesx.IsTestFile(pass, id.Pos()) {
			pass.Reportf(id.Pos(), "time.%s reads the wall clock in a test; inject a clock and give the test a fixed instant", obj.Name())
		}
	})
	return struct{}{}, nil
}
