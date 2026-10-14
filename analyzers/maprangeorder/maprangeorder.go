// Package maprangeorder is the bundled analyzer gqaci_map_range_order: map iteration order is
// random, so it must not decide the order of output.
package maprangeorder

import (
	"go/ast"
	"go/token"
	"go/types"
	"reflect"
	"slices"

	"golang.org/x/tools/go/analysis"

	"github.com/LongTermSupport/go-qa-ci/analyzers/internal/typesx"
)

// Name is the analyzer name: the identifier every finding is printed under.
const Name = "gqaci_map_range_order"

const doc = `gqaci_map_range_order forbids letting map iteration order decide the order of output

Flags a range over a map (a map type, including a defined one) in two situations, and only these:

1. its body writes output: fmt.Print*, fmt.Fprint*, log.Print*, or a Write, WriteString, WriteByte
   or WriteRune method of any type;
2. its body appends to a slice declared outside the loop (s = append(s, ...)) and the slice is then
   returned (or is a named result) or is printed or joined (fmt, strings.Join), with no sort of
   that slice (a function of package sort or slices that takes it) after the loop.

Go randomises map iteration order on purpose. Output built in that order differs from run to run:
golden files flake, diffs between two runs show phantom changes, and the same input yields
different reports. Collect the keys, sort them, and iterate the sorted keys.

A slice that is only counted, searched or consumed by something order-insensitive is not flagged
because it never reaches a return or a printer; one that is returned to a caller that sorts it is
flagged, because the function's own result is order-undefined.`

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
			if fn.Body != nil {
				checkFunction(pass, fn.Type, fn.Body)
			}
		case *ast.FuncLit:
			checkFunction(pass, fn.Type, fn.Body)
		}
		return true
	})
	return struct{}{}, nil
}

// checkFunction judges the map ranges that belong to the function itself, not to a literal in it.
func checkFunction(pass *analysis.Pass, sig *ast.FuncType, body *ast.BlockStmt) {
	results := resultVars(pass, sig)
	walkOwn(body, func(node ast.Node) {
		rng, ok := node.(*ast.RangeStmt)
		if !ok || !isMap(pass.TypesInfo.TypeOf(rng.X)) {
			return
		}
		if writesOutput(pass, rng.Body) {
			pass.Reportf(rng.For, "this loop ranges over a map and writes output in map order, which differs on every run; sort the keys first")
			return
		}
		for _, slice := range appendedSlices(pass, rng) {
			if !sortedAfter(pass, body, slice, rng.End()) && escapes(pass, body, slice, rng.End(), results) {
				pass.Reportf(rng.For, "this loop ranges over a map and appends to %s, which is never sorted before it is returned or printed; the order differs on every run", slice.Name())
				return
			}
		}
	})
}

// walkOwn visits every node of body except the insides of function literals.
func walkOwn(body ast.Node, visit func(ast.Node)) {
	ast.Inspect(body, func(node ast.Node) bool {
		if _, ok := node.(*ast.FuncLit); ok {
			return false
		}
		if node != nil {
			visit(node)
		}
		return true
	})
}

func isMap(t types.Type) bool {
	if t == nil {
		return false
	}
	_, ok := t.Underlying().(*types.Map)
	return ok
}

func resultVars(pass *analysis.Pass, sig *ast.FuncType) map[types.Object]bool {
	named := map[types.Object]bool{}
	if sig.Results == nil {
		return named
	}
	for _, field := range sig.Results.List {
		for _, name := range field.Names {
			if obj := pass.TypesInfo.Defs[name]; obj != nil {
				named[obj] = true
			}
		}
	}
	return named
}

// writesOutput says whether the loop body writes to a printer or a writer.
func writesOutput(pass *analysis.Pass, body *ast.BlockStmt) bool {
	found := false
	walkOwn(body, func(node ast.Node) {
		if call, ok := node.(*ast.CallExpr); ok {
			if fn := typesx.Callee(pass, call); fn != nil && isOutput(fn) {
				found = true
			}
		}
	})
	return found
}

var writeMethods = []string{"Write", "WriteString", "WriteByte", "WriteRune"}

func isOutput(fn *types.Func) bool {
	if fn.Pkg() == nil {
		return false
	}
	if sig, ok := fn.Type().(*types.Signature); ok && sig.Recv() != nil {
		return slices.Contains(writeMethods, fn.Name())
	}
	switch fn.Pkg().Path() {
	case "fmt":
		return startsWithAny(fn.Name(), "Print", "Fprint")
	case "log":
		return startsWithAny(fn.Name(), "Print")
	}
	return false
}

func startsWithAny(name string, prefixes ...string) bool {
	for _, prefix := range prefixes {
		if len(name) >= len(prefix) && name[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

// appendedSlices lists the variables declared before the loop that its body grows with
// s = append(s, ...).
func appendedSlices(pass *analysis.Pass, rng *ast.RangeStmt) []*types.Var {
	var out []*types.Var
	walkOwn(rng.Body, func(node ast.Node) {
		assign, ok := node.(*ast.AssignStmt)
		if !ok || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
			return
		}
		call, ok := assign.Rhs[0].(*ast.CallExpr)
		if !ok || len(call.Args) == 0 || !isBuiltinAppend(pass, call) {
			return
		}
		target, ok := assign.Lhs[0].(*ast.Ident)
		first, ok2 := call.Args[0].(*ast.Ident)
		if !ok || !ok2 {
			return
		}
		variable, isVar := pass.TypesInfo.ObjectOf(target).(*types.Var)
		if isVar && variable == pass.TypesInfo.ObjectOf(first) && variable.Pos() < rng.Pos() && !slices.Contains(out, variable) {
			out = append(out, variable)
		}
	})
	return out
}

func isBuiltinAppend(pass *analysis.Pass, call *ast.CallExpr) bool {
	id, ok := call.Fun.(*ast.Ident)
	if !ok {
		return false
	}
	builtin, ok := pass.TypesInfo.Uses[id].(*types.Builtin)
	return ok && builtin.Name() == "append"
}

// sortedAfter says whether, after the loop, a function of sort or slices takes the slice.
func sortedAfter(pass *analysis.Pass, body *ast.BlockStmt, slice *types.Var, after token.Pos) bool {
	found := false
	walkAll(body, func(call *ast.CallExpr) {
		fn := typesx.Callee(pass, call)
		if call.Pos() < after || fn == nil || fn.Pkg() == nil || (fn.Pkg().Path() != "sort" && fn.Pkg().Path() != "slices") {
			return
		}
		for _, arg := range call.Args {
			if mentions(pass, arg, slice) {
				found = true
			}
		}
	})
	return found
}

// escapes says whether, after the loop, the slice is returned, is a named result, or is passed to
// a printer or to strings.Join.
func escapes(pass *analysis.Pass, body *ast.BlockStmt, slice *types.Var, after token.Pos, results map[types.Object]bool) bool {
	if results[slice] {
		return true
	}
	found := false
	ast.Inspect(body, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.FuncLit:
			return false
		case *ast.ReturnStmt:
			for _, result := range n.Results {
				if n.Pos() >= after && carries(pass, result, slice) {
					found = true
				}
			}
		case *ast.CallExpr:
			if fn := typesx.Callee(pass, n); n.Pos() >= after && fn != nil && (isOutput(fn) || typesx.IsPkgFunc(fn, "strings", "Join")) {
				for _, arg := range n.Args {
					if carries(pass, arg, slice) {
						found = true
					}
				}
			}
		}
		return true
	})
	return found
}

// walkAll visits the calls in body, function literals included: a sort inside a closure counts.
func walkAll(body ast.Node, visit func(*ast.CallExpr)) {
	ast.Inspect(body, func(node ast.Node) bool {
		if call, ok := node.(*ast.CallExpr); ok {
			visit(call)
		}
		return true
	})
}

// mentions says whether expr refers to the variable.
func mentions(pass *analysis.Pass, expr ast.Expr, variable *types.Var) bool {
	found := false
	ast.Inspect(expr, func(node ast.Node) bool {
		if id, ok := node.(*ast.Ident); ok && pass.TypesInfo.ObjectOf(id) == variable {
			found = true
		}
		return !found
	})
	return found
}

// carries says whether expr hands the slice itself on: the variable, a reslice of it, or a
// composite literal holding it. len(s) and s[i] do not carry the order.
func carries(pass *analysis.Pass, expr ast.Expr, variable *types.Var) bool {
	switch e := ast.Unparen(expr).(type) {
	case *ast.Ident:
		return pass.TypesInfo.ObjectOf(e) == variable
	case *ast.SliceExpr:
		return carries(pass, e.X, variable)
	case *ast.CompositeLit:
		return slices.ContainsFunc(e.Elts, func(elt ast.Expr) bool { return carries(pass, elt, variable) })
	case *ast.KeyValueExpr:
		return carries(pass, e.Value, variable)
	}
	return false
}
