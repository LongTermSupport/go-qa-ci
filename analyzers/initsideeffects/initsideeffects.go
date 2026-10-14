// Package initsideeffects is the bundled analyzer gqaci_init_side_effects: init functions only
// prepare package variables.
package initsideeffects

import (
	"go/ast"
	"go/types"
	"reflect"
	"slices"

	"golang.org/x/tools/go/analysis"

	"github.com/LongTermSupport/go-qa-ci/analyzers/internal/typesx"
)

// Name is the analyzer name: the identifier every finding is printed under.
const Name = "gqaci_init_side_effects"

const doc = `gqaci_init_side_effects forbids side effects in init functions

Flags, inside the body of an init function, a go statement and a call (also one made by a function
literal called at once) of a function or method that does input or output or changes the process:
anything in os except the environment and process-identity readers, os/exec, os/signal, os/user,
io/ioutil, syscall, net, net/rpc, net/smtp, database/sql (except Register and Drivers), the client
and server of net/http, fmt.Print, fmt.Printf and fmt.Println, flag.Parse, and time.Sleep and the
timers. Assigning package variables, compiling a regular expression, building a table and
registering a handler or driver are not flagged, and neither is a function literal that is only
stored.

An init function runs at import, in an order no caller chose, before main and before any test, in
every binary that links the package, including tools that only wanted a type from it. A package
that does input or output at import cannot be imported safely, its failure has no caller to
report to, and it cannot be tested without running it. Move the work into a function the caller
invokes.`

// Analyzer is the analyzer.
var Analyzer = &analysis.Analyzer{
	Name:       Name,
	Doc:        doc,
	ResultType: reflect.TypeFor[struct{}](),
	Run:        run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Name.Name == "init" && fn.Body != nil {
				inspectBody(pass, fn.Body)
			}
		}
	}
	return struct{}{}, nil
}

// inspectBody reports the side effects the body performs when it runs, which excludes a function
// literal that is only created.
func inspectBody(pass *analysis.Pass, body ast.Node) {
	ast.Inspect(body, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.FuncLit:
			return false
		case *ast.GoStmt:
			pass.Reportf(n.Pos(), "init starts a goroutine at import; start it from a function the caller invokes")
		case *ast.CallExpr:
			if lit, ok := ast.Unparen(n.Fun).(*ast.FuncLit); ok {
				inspectBody(pass, lit.Body)
			}
			if fn := typesx.Callee(pass, n); fn != nil {
				if label := sideEffect(fn); label != "" {
					pass.Reportf(n.Pos(), "init performs %s at import; do the work in a function the caller invokes", label)
				}
			}
		}
		return true
	})
}

// pure lists the functions of os that only read the environment or the identity of the process.
var pure = []string{
	"Getenv", "LookupEnv", "Environ", "ExpandEnv", "Expand", "Getpid", "Getppid", "Getuid", "Getgid",
	"Geteuid", "Getegid", "UserHomeDir", "UserCacheDir", "UserConfigDir", "TempDir",
	"IsNotExist", "IsExist", "IsPermission", "IsTimeout", "NewSyscallError", "NewFile",
}

var ioPackages = []string{"os/exec", "os/signal", "os/user", "io/ioutil", "syscall", "net", "net/rpc", "net/smtp", "net/textproto"}

// sideEffect names the function when calling it does input or output or changes the process.
func sideEffect(fn *types.Func) string {
	if fn.Pkg() == nil {
		return ""
	}
	path, name := fn.Pkg().Path(), fn.Name()
	label := path[lastSlash(path)+1:] + "." + name
	if sig, ok := fn.Type().(*types.Signature); ok && sig.Recv() != nil {
		label = typesx.ReceiverName(sig) + "." + name
	}
	switch {
	case path == "os":
		if !slices.Contains(pure, name) {
			return label
		}
	case slices.Contains(ioPackages, path):
		return label
	case path == "database/sql":
		if !slices.Contains([]string{"Register", "Drivers"}, name) {
			return label
		}
	case path == "net/http":
		if slices.Contains([]string{"Get", "Head", "Post", "PostForm", "ListenAndServe", "ListenAndServeTLS", "Serve", "ServeTLS", "ServeFile", "ServeContent"}, name) ||
			slices.Contains([]string{"Client", "Server", "Transport"}, receiver(fn)) {
			return label
		}
	case path == "fmt":
		if slices.Contains([]string{"Print", "Printf", "Println"}, name) {
			return label
		}
	case path == "flag":
		if name == "Parse" {
			return label
		}
	case path == "time":
		if slices.Contains([]string{"Sleep", "AfterFunc", "NewTicker", "NewTimer", "Tick", "After"}, name) {
			return label
		}
	}
	return ""
}

func receiver(fn *types.Func) string {
	if sig, ok := fn.Type().(*types.Signature); ok {
		return typesx.ReceiverName(sig)
	}
	return ""
}

func lastSlash(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '/' {
			return i
		}
	}
	return -1
}
