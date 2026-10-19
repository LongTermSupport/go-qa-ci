package runners

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
)

// buildEnv is the environment of the build: the caller's GOFLAGS, from `go env` so that flags set
// with `go env -w` count too, minus any -mod flag. With none, the go command uses vendor/ when it
// exists and -mod=readonly otherwise, so the build judges the program the consumer ships, never
// rewrites go.mod or go.sum and never fetches. An explicit -mod=readonly would ignore vendor/, and a
// -mod=mod from the caller would rewrite go.mod. The workspace and tags stay the caller's.
func buildEnv(ctx context.Context, dir string) ([]string, error) {
	out, err := Capture(ctx, dir, goBinary, "env", "GOFLAGS")
	if err != nil {
		return nil, err
	}
	if out.Code != 0 {
		return nil, fmt.Errorf("go env GOFLAGS exited %d: %s", out.Code, strings.TrimSpace(out.Combined()))
	}
	kept := slices.DeleteFunc(strings.Fields(out.Stdout), func(flag string) bool { return strings.HasPrefix(flag, "-mod=") })
	return []string{"GOFLAGS=" + strings.Join(kept, " ")}, nil
}

// testOnlyMark is what buildable prints after a package that loads and has only test files.
const testOnlyMark = "test-only"

// buildable resolves the named package patterns to the import paths go build is given, leaving out
// every test-only package: one that loads and has no production file. go build refuses such a
// package when it is named ("no non-test Go files"), although ./... skips it, and its test files
// are compiled by the test stage. A package that does not load is kept, so go build reports it.
// passed is false when go list itself failed; output is then everything it printed.
func buildable(ctx context.Context, dir string, env, packages []string) (kept []string, output string, passed bool, err error) {
	format := "{{.ImportPath}}{{if and (not .Error) (not .GoFiles) (not .CgoFiles) (or .TestGoFiles .XTestGoFiles)}} " + testOnlyMark + "{{end}}"
	out, err := CaptureEnv(ctx, dir, env, goBinary, slices.Concat([]string{"list", "-e", "-f", format}, packages)...)
	if err != nil {
		return nil, "", false, err
	}
	if out.Code != 0 {
		return nil, out.Combined(), false, nil
	}
	for line := range strings.Lines(out.Stdout) {
		fields := strings.Fields(line)
		if len(fields) == 1 {
			kept = append(kept, fields[0])
		}
	}
	return kept, "", true, nil
}

// Build compiles the packages of the module in dir without their test files, so a production file
// that needs a helper defined only in a _test.go file fails here although vet, the linters and the
// tests, which all load test files with the package, accept it. passed is false when the code did
// not compile; output is then everything go build printed. The given package patterns are built,
// less any test-only package among them, or every package when there are none.
func Build(ctx context.Context, dir string, packages ...string) (output string, passed bool, err error) {
	if err = Locate(goBinary); err != nil {
		return "", false, err
	}
	env, err := buildEnv(ctx, dir)
	if err != nil {
		return "", false, err
	}
	if len(packages) > 0 {
		packages, output, passed, err = buildable(ctx, dir, env, packages)
		if err != nil || !passed {
			return output, passed, err
		}
		if len(packages) == 0 {
			return "", true, nil
		}
	}
	// -o os.DevNull keeps the binary of a main package out of the module.
	out, err := CaptureEnv(ctx, dir, env, goBinary, slices.Concat([]string{"build", "-o", os.DevNull}, orAll(packages))...)
	if err != nil {
		return "", false, err
	}
	return out.Combined(), out.Code == 0, nil
}
