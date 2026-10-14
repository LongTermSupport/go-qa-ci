// Package execcontext is the bundled analyzer gqaci_exec_context: os/exec.Command must not be used
// where a context is in scope, because a process started without the context outlives its
// cancellation.
package execcontext

import (
	"go/ast"
	"go/token"
	"go/types"
	"reflect"

	"golang.org/x/tools/go/analysis"

	"github.com/LongTermSupport/go-qa-ci/analyzers/internal/typesx"
)

// Name is the analyzer name: the identifier every finding is printed under.
const Name = "gqaci_exec_context"

const doc = `gqaci_exec_context forbids exec.Command where a context is in scope

Flags any use of os/exec.Command (a call, a method value, a function stored in a variable) inside
a function that has a context.Context, or a *testing.T, *testing.B, *testing.F or testing.TB, in
scope: a parameter, a receiver or a local variable declared before the use, in the function or in
one that encloses it.

A command started with exec.Command ignores the context's cancellation and deadline: the caller
gives up, the child keeps running, and a timeout cannot stop a hung process. Use
exec.CommandContext(ctx, ...), or t.Context() in a test.`

// Analyzer is the analyzer.
var Analyzer = &analysis.Analyzer{
	Name:       Name,
	Doc:        doc,
	ResultType: reflect.TypeFor[struct{}](),
	Run:        run,
}

func run(pass *analysis.Pass) (any, error) {
	typesx.EachUse(pass, func(id *ast.Ident, obj types.Object, _ []ast.Node) {
		if !typesx.IsPkgFunc(obj, "os/exec", "Command") {
			return
		}
		if name := contextInScope(pass, id.Pos()); name != "" {
			pass.Reportf(id.Pos(), "exec.Command is used while context %s is in scope; use exec.CommandContext so cancellation stops the process", name)
		}
	})
	return struct{}{}, nil
}

// contextInScope names a variable holding a context that is visible at pos, or returns "".
func contextInScope(pass *analysis.Pass, pos token.Pos) string {
	pkgScope := pass.Pkg.Scope()
	for scope := pkgScope.Innermost(pos); scope != nil && scope != pkgScope && scope != types.Universe; scope = scope.Parent() {
		for _, name := range scope.Names() {
			variable, ok := scope.Lookup(name).(*types.Var)
			if ok && variable.Pos() < pos && carriesContext(variable.Type()) {
				return name
			}
		}
	}
	return ""
}

func carriesContext(t types.Type) bool {
	return typesx.NamedIs(t, "context", "Context") || typesx.NamedIs(t, "testing", "T", "B", "F", "TB")
}
