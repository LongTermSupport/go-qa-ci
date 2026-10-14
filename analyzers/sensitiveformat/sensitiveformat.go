// Package sensitiveformat is the bundled analyzer gqaci_sensitive_format: a value that is, or
// holds, a sensitive-named type must not be printed with fmt, log or log/slog unless its type
// redacts itself. Aliases, embedding, generics and method sets decide it, so it needs type
// information and is a go/analysis analyzer run through go vet.
package sensitiveformat

import (
	"errors"
	"flag"
	"go/ast"
	"go/constant"
	"go/types"
	"reflect"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/types/typeutil"
)

// Name is the analyzer name: the identifier every finding is printed under.
const Name = "gqaci_sensitive_format"

const doc = `gqaci_sensitive_format forbids printing a sensitive value without redaction

Flags a value whose type is, or holds through aliases, embedding, fields, slices, arrays, maps or
generic type arguments, a type with a sensitive name (password, secret, token, credential, apikey,
privatekey by default) when it is passed to the %v, %+v, %#v, %s or %w verb of a fmt function, to a
log function, or to a log/slog call, and the type has no redacting method.

Which method redacts depends on the destination: String, Error or Format(fmt.State, rune) for %v,
%+v, %s and %w; GoString or Format for %#v; LogValue() slog.Value or Error for slog (slog never
calls String, and a JSON handler marshals the fields). A type listed with -redacting redacts
everywhere. fmt calls the methods of a nested field; slog resolves only the top-level value.

Fix it by giving the type a String method and a LogValue method that print a fixed placeholder
(and a GoString method if it is printed with %#v), or by passing only the non-sensitive fields.`

// Config is what the analyzer is told about the project.
type Config struct {
	// Patterns are lower-case name fragments that mark a type as sensitive.
	Patterns []string
	// Redacting are fully qualified type names (path.Name) that are redacting without a method.
	Redacting []string
}

// DefaultConfig is the configuration of the registered Analyzer.
func DefaultConfig() Config {
	return Config{Patterns: []string{"password", "secret", "token", "credential", "apikey", "privatekey"}}
}

// Analyzer is the analyzer with the default configuration.
var Analyzer = New(DefaultConfig())

type checker struct {
	patterns  []string
	redacting map[string]bool
}

// New returns the analyzer configured with cfg. The configuration is also settable through the
// -patterns and -redacting flags, whose defaults are cfg.
func New(cfg Config) *analysis.Analyzer {
	settings := Config{Patterns: slices.Clone(cfg.Patterns), Redacting: slices.Clone(cfg.Redacting)}
	analyzer := &analysis.Analyzer{
		Name:       Name,
		Doc:        doc,
		ResultType: reflect.TypeFor[struct{}](),
	}
	analyzer.Flags.Var(&listFlag{values: &settings.Patterns, required: true}, "patterns",
		"comma-separated, case-insensitive name fragments that mark a type as sensitive")
	analyzer.Flags.Var(&listFlag{values: &settings.Redacting}, "redacting",
		"comma-separated fully qualified types (path.Name) treated as redacting")
	analyzer.Run = func(pass *analysis.Pass) (any, error) {
		check := &checker{redacting: map[string]bool{}}
		for _, pattern := range settings.Patterns {
			check.patterns = append(check.patterns, normalise(pattern))
		}
		for _, name := range settings.Redacting {
			check.redacting[name] = true
		}
		for _, file := range pass.Files {
			ast.Inspect(file, func(node ast.Node) bool {
				if call, ok := node.(*ast.CallExpr); ok {
					check.call(pass, call)
				}
				return true
			})
		}
		return struct{}{}, nil
	}
	return analyzer
}

// listFlag is a comma-separated list flag.
type listFlag struct {
	values   *[]string
	required bool
}

var errEmptyList = errors.New("the list must not be empty: an empty pattern list switches the rule off")

func (f *listFlag) String() string {
	if f == nil || f.values == nil {
		return ""
	}
	return strings.Join(*f.values, ",")
}

func (f *listFlag) Set(value string) error {
	var parsed []string
	for part := range strings.SplitSeq(value, ",") {
		if part = strings.TrimSpace(part); part != "" {
			parsed = append(parsed, part)
		}
	}
	if f.required && len(parsed) == 0 {
		return errEmptyList
	}
	*f.values = parsed
	return nil
}

var _ flag.Value = (*listFlag)(nil)

// call checks the operands of one call to a printing function.
func (c *checker) call(pass *analysis.Pass, call *ast.CallExpr) {
	fn, ok := typeutil.Callee(pass.TypesInfo, call).(*types.Func)
	if !ok || fn.Pkg() == nil || call.Ellipsis.IsValid() {
		return
	}
	sig, ok := fn.Type().(*types.Signature)
	if !ok {
		return
	}
	switch fn.Pkg().Path() {
	case fmtPath:
		if isPrinter(fn.Name()) {
			c.printf(pass, call, sig)
		}
	case "log":
		c.printf(pass, call, sig)
	case slogPath:
		c.slog(pass, call, sig)
	}
}

func isPrinter(name string) bool {
	for _, prefix := range []string{"Print", "Sprint", "Fprint", "Append", "Errorf"} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// variadicAny returns the index of the trailing ...any parameter, or -1.
func variadicAny(sig *types.Signature) int {
	if !sig.Variadic() {
		return -1
	}
	last := sig.Params().Len() - 1
	slice, ok := sig.Params().At(last).Type().(*types.Slice)
	if !ok || !isEmptyInterface(slice.Elem()) {
		return -1
	}
	return last
}

func isEmptyInterface(t types.Type) bool {
	iface, ok := types.Unalias(t).Underlying().(*types.Interface)
	return ok && iface.Empty()
}

// printf checks a fmt or log call: a Print-style call formats every operand with %v, a
// Printf-style call formats each with the verb the format string names.
func (c *checker) printf(pass *analysis.Pass, call *ast.CallExpr, sig *types.Signature) {
	first := variadicAny(sig)
	if first < 0 || len(call.Args) < first {
		return
	}
	params := sig.Params()
	var verbs map[int][]verb
	formatted := first >= 1 && params.At(first-1).Name() == "format" && isBasic(params.At(first-1).Type(), types.String)
	if formatted {
		format := pass.TypesInfo.Types[call.Args[first-1]]
		if format.Value != nil && format.Value.Kind() == constant.String {
			verbs = verbsByArg(constant.StringVal(format.Value))
		}
	}
	for index, arg := range call.Args[first:] {
		for _, v := range operandVerbs(formatted, verbs, index) {
			if v.letter == 'v' || v.letter == 's' || v.letter == 'w' {
				c.report(pass, arg, fmtSink(v))
			}
		}
	}
}

// operandVerbs lists the verbs formatting operand index. A Print-style call and a format string
// that is not a constant are read as %v, which is the cautious reading.
func operandVerbs(formatted bool, verbs map[int][]verb, index int) []verb {
	if !formatted || verbs == nil {
		return []verb{{letter: 'v'}}
	}
	return verbs[index]
}

// slog checks every operand of a log/slog function or method passed as an any: the key/value
// arguments of Info and its siblings, and the value of Any and AnyValue.
func (c *checker) slog(pass *analysis.Pass, call *ast.CallExpr, sig *types.Signature) {
	params := sig.Params()
	for index, arg := range call.Args {
		param := min(index, params.Len()-1)
		paramType := params.At(param).Type()
		if sig.Variadic() && param == params.Len()-1 {
			if slice, ok := paramType.(*types.Slice); ok {
				paramType = slice.Elem()
			}
		}
		if isEmptyInterface(paramType) {
			c.report(pass, arg, slogSink)
		}
	}
}

func (c *checker) report(pass *analysis.Pass, arg ast.Expr, to sink) {
	typ := pass.TypesInfo.TypeOf(arg)
	if typ == nil {
		return
	}
	if found := c.exposed(typ, to); found != nil {
		pass.Reportf(arg.Pos(), "%s exposes sensitive type %s to %s and implements no redacting %s method",
			types.TypeString(typ, packageName), types.TypeString(found, packageName), to.destination, to.methods)
	}
}

func packageName(pkg *types.Package) string { return pkg.Name() }
