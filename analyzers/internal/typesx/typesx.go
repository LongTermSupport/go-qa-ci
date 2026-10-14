// Package typesx holds the type-based helpers the bundled analyzers share. Every helper resolves
// a go/types object, so a rule keyed on it follows an import alias, a dot import, a method value
// and a function stored in a variable, where a rule keyed on the spelling of a name does not.
package typesx

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// Inspect walks every file of the pass and calls visit with each node and the stack of its
// ancestors (outermost first, not including the node). Returning false skips the node's children.
func Inspect(pass *analysis.Pass, visit func(node ast.Node, stack []ast.Node) bool) {
	for _, file := range pass.Files {
		var stack []ast.Node
		ast.Inspect(file, func(node ast.Node) bool {
			if node == nil {
				stack = stack[:len(stack)-1]
				return true
			}
			descend := visit(node, stack)
			if descend {
				stack = append(stack, node)
			}
			return descend
		})
	}
}

// EachUse calls visit for every identifier that refers to an object, in source order: a call, a
// method value, a function assigned to a variable and a bare reference are all uses.
func EachUse(pass *analysis.Pass, visit func(id *ast.Ident, obj types.Object, stack []ast.Node)) {
	Inspect(pass, func(node ast.Node, stack []ast.Node) bool {
		if id, ok := node.(*ast.Ident); ok {
			if obj := pass.TypesInfo.Uses[id]; obj != nil {
				visit(id, obj, stack)
			}
		}
		return true
	})
}

// IsTestFile reports whether pos lies in a _test.go file.
func IsTestFile(pass *analysis.Pass, pos token.Pos) bool {
	return strings.HasSuffix(pass.Fset.Position(pos).Filename, "_test.go")
}

// IsPkgFunc reports whether obj is the package-level function path.name for one of names.
func IsPkgFunc(obj types.Object, path string, names ...string) bool {
	fn, ok := obj.(*types.Func)
	if !ok || fn.Pkg() == nil || fn.Pkg().Path() != path {
		return false
	}
	sig, ok := fn.Type().(*types.Signature)
	return ok && sig.Recv() == nil && slices.Contains(names, fn.Name())
}

// IsMethod reports whether obj is a method named one of names, declared in package path on a
// receiver type named recv (empty recv accepts any receiver of that package).
func IsMethod(obj types.Object, path, recv string, names ...string) bool {
	fn, ok := obj.(*types.Func)
	if !ok || fn.Pkg() == nil || fn.Pkg().Path() != path || !slices.Contains(names, fn.Name()) {
		return false
	}
	sig, ok := fn.Type().(*types.Signature)
	if !ok || sig.Recv() == nil {
		return false
	}
	return recv == "" || ReceiverName(sig) == recv
}

// ReceiverName is the name of the named type a method is declared on, without its pointer.
func ReceiverName(sig *types.Signature) string {
	if sig.Recv() == nil {
		return ""
	}
	t := sig.Recv().Type()
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}
	if named, ok := types.Unalias(t).(*types.Named); ok {
		return named.Obj().Name()
	}
	return ""
}

// Callee is the function or method a call expression calls, or nil for a call of a function value
// or a conversion.
func Callee(pass *analysis.Pass, call *ast.CallExpr) *types.Func {
	var id *ast.Ident
	switch fun := ast.Unparen(call.Fun).(type) {
	case *ast.Ident:
		id = fun
	case *ast.SelectorExpr:
		id = fun.Sel
	case *ast.IndexExpr:
		return Callee(pass, &ast.CallExpr{Fun: fun.X})
	case *ast.IndexListExpr:
		return Callee(pass, &ast.CallExpr{Fun: fun.X})
	default:
		return nil
	}
	fn, ok := pass.TypesInfo.Uses[id].(*types.Func)
	if !ok {
		return nil
	}
	return fn.Origin()
}

// ConstString is the value of expr when it is a constant string, whatever spells it: a literal, a
// named constant or a constant expression.
func ConstString(pass *analysis.Pass, expr ast.Expr) (string, bool) {
	tv, ok := pass.TypesInfo.Types[expr]
	if !ok || tv.Value == nil || tv.Value.Kind() != constant.String {
		return "", false
	}
	return constant.StringVal(tv.Value), true
}

// NamedIs reports whether t, without pointers and aliases, is the named type path.name.
func NamedIs(t types.Type, path string, names ...string) bool {
	for {
		ptr, ok := types.Unalias(t).(*types.Pointer)
		if !ok {
			break
		}
		t = ptr.Elem()
	}
	named, ok := types.Unalias(t).(*types.Named)
	if !ok || named.Obj().Pkg() == nil {
		return false
	}
	return named.Obj().Pkg().Path() == path && slices.Contains(names, named.Obj().Name())
}
