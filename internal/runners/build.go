package runners

import (
	"context"
	"os"
	"slices"
	"strings"
)

// buildEnv makes the build independent of the consumer's workspace, as the tool installer does:
// GOWORK=off so a go.work cannot change what the module resolves, and -mod=mod so a vendor
// directory cannot hide a missing requirement. The consumer's own GOFLAGS (build tags) stay.
func buildEnv() []string {
	flags := strings.TrimSpace(os.Getenv("GOFLAGS") + " -mod=mod")
	return []string{"GOFLAGS=" + flags, "GOWORK=off"}
}

// Build compiles the packages of the module in dir without their test files, so a production file
// that needs a helper defined only in a _test.go file fails here although vet, the linters and the
// tests, which all load test files with the package, accept it. passed is false when the code did
// not compile; output is then everything go build printed. The given package patterns are built,
// or every package when there are none.
func Build(ctx context.Context, dir string, packages ...string) (output string, passed bool, err error) {
	if err = Locate(goBinary); err != nil {
		return "", false, err
	}
	out, err := CaptureEnv(ctx, dir, buildEnv(), goBinary, slices.Concat([]string{"build"}, orAll(packages))...)
	if err != nil {
		return "", false, err
	}
	return out.Combined(), out.Code == 0, nil
}
