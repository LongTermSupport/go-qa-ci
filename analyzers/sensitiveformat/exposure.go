package sensitiveformat

import (
	"go/types"
	"strings"
)

const (
	fmtPath  = "fmt"
	slogPath = "log/slog"
)

// sink is where a value is printed. What redacts a value depends on it: fmt calls String and Error
// for %v and %s but GoString only for %#v, and never LogValue; slog calls LogValue and Error but
// never String, because the handler decides the encoding and a JSON handler marshals the fields.
type sink struct {
	destination string
	methods     string
	redactedBy  func(*types.MethodSet) bool
	// nested says whether the sink calls the methods of a value reached through a field, element
	// or key. fmt does; slog resolves only the top-level value.
	nested bool
}

func fmtSink(v verb) sink {
	if v.sharp {
		return sink{
			destination: "%#" + string(v.letter), methods: "GoString or Format", nested: true,
			redactedBy: func(m *types.MethodSet) bool { return hasStringMethod(m, "GoString") || hasFormat(m) },
		}
	}
	return sink{
		destination: "%" + string(v.letter), methods: "String, Error or Format", nested: true,
		redactedBy: func(m *types.MethodSet) bool {
			return hasStringMethod(m, "String") || hasStringMethod(m, "Error") || hasFormat(m)
		},
	}
}

var slogSink = sink{
	destination: "slog", methods: "LogValue or Error",
	redactedBy: func(m *types.MethodSet) bool { return hasLogValue(m) || hasStringMethod(m, "Error") },
}

// holdKey identifies a type visited at a given redaction visibility, so a recursive type ends.
type holdKey struct {
	named    *types.Named
	honoured bool
}

// exposed returns the sensitive named type that formatting a value of type t would print, or nil
// when nothing sensitive is printed: either no sensitive type is held, or the type that holds it
// redacts itself.
func (c *checker) exposed(t types.Type, to sink) *types.Named {
	t = types.Unalias(t)
	if c.redacts(t, to) {
		return nil
	}
	if ptr, ok := t.(*types.Pointer); ok {
		// fmt prints a pointer to a struct, array, slice or map as &{...}; only that top level is
		// followed, because a pointer nested in a value prints as an address.
		return c.holds(ptr.Elem(), true, map[holdKey]bool{}, to)
	}
	return c.holds(t, true, map[holdKey]bool{}, to)
}

// holds finds a sensitive named type in t. honoured says whether fmt would call t's formatting
// methods: it does not for a value reached through an unexported field, which is why a redacting
// type kept in an unexported field still prints in full.
func (c *checker) holds(t types.Type, honoured bool, seen map[holdKey]bool, to sink) *types.Named {
	t = types.Unalias(t)
	switch typ := t.(type) {
	case *types.Named:
		key := holdKey{typ, honoured}
		if seen[key] {
			return nil
		}
		seen[key] = true
		if honoured && c.redacts(typ, to) {
			return nil
		}
		if c.sensitiveName(typ) {
			return typ
		}
		return c.holds(typ.Underlying(), honoured, seen, to)
	case *types.Struct:
		for field := range typ.Fields() {
			if found := c.holds(field.Type(), honoured && to.nested && field.Exported(), seen, to); found != nil {
				return found
			}
		}
	case *types.Slice:
		return c.holds(typ.Elem(), honoured && to.nested, seen, to)
	case *types.Array:
		return c.holds(typ.Elem(), honoured && to.nested, seen, to)
	case *types.Map:
		if found := c.holds(typ.Key(), honoured && to.nested, seen, to); found != nil {
			return found
		}
		return c.holds(typ.Elem(), honoured && to.nested, seen, to)
	}
	return nil
}

// sensitiveName reports whether the type's own name contains a configured pattern, ignoring case
// and underscores.
func (c *checker) sensitiveName(named *types.Named) bool {
	name := normalise(named.Obj().Name())
	for _, pattern := range c.patterns {
		if strings.Contains(name, pattern) {
			return true
		}
	}
	return false
}

func normalise(name string) string {
	return strings.ToLower(strings.ReplaceAll(name, "_", ""))
}

func qualified(named *types.Named) string {
	if named.Obj().Pkg() == nil {
		return named.Obj().Name()
	}
	return named.Obj().Pkg().Path() + "." + named.Obj().Name()
}

// redacts reports whether the sink would use a method of t instead of printing its contents, or
// the project declared the type redacting.
func (c *checker) redacts(t types.Type, to sink) bool {
	base := t
	if ptr, ok := base.(*types.Pointer); ok {
		base = ptr.Elem()
	}
	if named, ok := types.Unalias(base).(*types.Named); ok && c.redacting[qualified(named)] {
		return true
	}
	return to.redactedBy(types.NewMethodSet(t))
}

func lookupSignature(methods *types.MethodSet, name string) *types.Signature {
	selection := methods.Lookup(nil, name)
	if selection == nil {
		return nil
	}
	sig, ok := selection.Type().(*types.Signature)
	if !ok {
		return nil
	}
	return sig
}

func hasStringMethod(methods *types.MethodSet, name string) bool {
	sig := lookupSignature(methods, name)
	return sig != nil && sig.Params().Len() == 0 && sig.Results().Len() == 1 && isBasic(sig.Results().At(0).Type(), types.String)
}

func hasFormat(methods *types.MethodSet) bool {
	sig := lookupSignature(methods, "Format")
	if sig == nil || sig.Params().Len() != 2 || sig.Results().Len() != 0 {
		return false
	}
	return isNamed(sig.Params().At(0).Type(), fmtPath, "State") && isBasic(sig.Params().At(1).Type(), types.Int32)
}

func hasLogValue(methods *types.MethodSet) bool {
	sig := lookupSignature(methods, "LogValue")
	return sig != nil && sig.Params().Len() == 0 && sig.Results().Len() == 1 && isNamed(sig.Results().At(0).Type(), slogPath, "Value")
}

func isBasic(t types.Type, kind types.BasicKind) bool {
	basic, ok := types.Unalias(t).(*types.Basic)
	return ok && basic.Kind() == kind
}

func isNamed(t types.Type, pkgPath, name string) bool {
	named, ok := types.Unalias(t).(*types.Named)
	return ok && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == pkgPath && named.Obj().Name() == name
}
