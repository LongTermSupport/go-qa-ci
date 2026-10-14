// Package shellexec is the bundled analyzer gqaci_shell_exec: a program is started directly, with
// an argument vector, not as a string the shell parses.
package shellexec

import (
	"go/ast"
	"path"
	"reflect"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/LongTermSupport/go-qa-ci/analyzers/internal/typesx"
)

// Name is the analyzer name: the identifier every finding is printed under.
const Name = "gqaci_shell_exec"

const doc = `gqaci_shell_exec forbids starting a shell with -c

Flags a call of exec.Command, exec.CommandContext, os.StartProcess or syscall.Exec whose program
is, by constant analysis (a literal, a named constant or a constant expression), a shell (sh, bash,
zsh, dash, ash, ksh, csh, tcsh, fish), directly or behind a wrapper such as env, sudo, nohup or
timeout, and whose arguments before the first one that is not an option contain -c, or a cluster
of short options ending in c such as -lc.

Handing the shell a command string moves the program and its arguments into one string that is
parsed again: quoting becomes the caller's job, an argument holding a space, quote, ; or $ changes
the command, and checks keyed on the argument vector (a command allowlist, a security linter, a
sandbox policy) see only "sh". Pass the program and its arguments as separate values. When a
pipeline is genuinely needed, connect the processes with their stdin and stdout.`

// Analyzer is the analyzer.
var Analyzer = &analysis.Analyzer{
	Name:       Name,
	Doc:        doc,
	ResultType: reflect.TypeFor[struct{}](),
	Run:        run,
}

var shells = []string{"sh", "bash", "zsh", "dash", "ash", "ksh", "csh", "tcsh", "fish"}

var wrappers = []string{"env", "sudo", "doas", "nohup", "nice", "timeout", "time", "stdbuf", "setsid", "xargs", "busybox"}

func run(pass *analysis.Pass) (any, error) {
	typesx.Inspect(pass, func(node ast.Node, _ []ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		if args, ok := argv(pass, call); ok {
			if shell, flag := shellWithC(pass, args); shell != "" {
				pass.Reportf(call.Pos(), "this call runs the shell %s with %s, which hides the real command from argument-based checks and from quoting; pass the program and its arguments separately", shell, flag)
			}
		}
		return true
	})
	return struct{}{}, nil
}

// argv returns the expressions that make up the program and its arguments, when the call is one
// that starts a process and the vector is written out in the call.
func argv(pass *analysis.Pass, call *ast.CallExpr) ([]ast.Expr, bool) {
	fn := typesx.Callee(pass, call)
	if fn == nil {
		return nil, false
	}
	switch {
	case typesx.IsPkgFunc(fn, "os/exec", "Command") && !call.Ellipsis.IsValid():
		return call.Args, len(call.Args) > 0
	case typesx.IsPkgFunc(fn, "os/exec", "CommandContext") && !call.Ellipsis.IsValid():
		return call.Args[min(1, len(call.Args)):], len(call.Args) > 1
	case typesx.IsPkgFunc(fn, "os", "StartProcess") && len(call.Args) >= 2:
		return literalVector(call.Args[0], call.Args[1])
	case typesx.IsPkgFunc(fn, "syscall", "Exec") && len(call.Args) >= 2:
		return literalVector(call.Args[0], call.Args[1])
	}
	return nil, false
}

// literalVector joins the program path with a []string literal holding the arguments; argv[0] of
// the literal repeats the program name and is skipped.
func literalVector(program, vector ast.Expr) ([]ast.Expr, bool) {
	lit, ok := ast.Unparen(vector).(*ast.CompositeLit)
	if !ok || len(lit.Elts) == 0 {
		return nil, false
	}
	return append([]ast.Expr{program}, lit.Elts[1:]...), true
}

// shellWithC finds the shell the vector starts, and the option that gives it a command string. A
// wrapper program (env, sudo, ...) may come first, followed by its own options and assignments.
func shellWithC(pass *analysis.Pass, args []ast.Expr) (shell, flag string) {
	for i, arg := range args {
		name, ok := typesx.ConstString(pass, arg)
		if !ok {
			return "", ""
		}
		base := path.Base(name)
		switch {
		case slices.Contains(shells, base):
			if flag = commandOption(pass, args[i+1:]); flag != "" {
				return base, flag
			}
			return "", ""
		case i == 0 && slices.Contains(wrappers, base):
		case i > 0 && (strings.HasPrefix(name, "-") || strings.Contains(name, "=")):
		default:
			return "", ""
		}
	}
	return "", ""
}

// commandOption is the -c style option among the leading options of the shell's arguments.
func commandOption(pass *analysis.Pass, args []ast.Expr) string {
	for _, arg := range args {
		value, ok := typesx.ConstString(pass, arg)
		if !ok || !strings.HasPrefix(value, "-") || value == "-" || value == "--" {
			return ""
		}
		if !strings.HasPrefix(value, "--") && strings.HasSuffix(value, "c") && isLetters(value[1:]) {
			return value
		}
	}
	return ""
}

func isLetters(s string) bool {
	return s != "" && strings.IndexFunc(s, func(r rune) bool { return (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') }) < 0
}
