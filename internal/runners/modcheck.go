package runners

import (
	"context"
	"fmt"
	"strings"
)

const goBinary = "go"

// Mod checks the module in dir: go.mod and go.sum must be what `go mod tidy` would write, and the
// downloaded dependencies must match their recorded hashes. It returns one problem per failed
// check, each a complete message; an error means a check could not be run.
func Mod(ctx context.Context, dir string) (problems []string, err error) {
	tidy, err := Capture(ctx, dir, goBinary, "mod", "tidy", "-diff")
	if err != nil {
		return nil, err
	}
	switch tidy.Code {
	case 0:
	case 1:
		if strings.TrimSpace(tidy.Stdout) == "" {
			return nil, fmt.Errorf("go mod tidy -diff failed: %s", strings.TrimSpace(tidy.Stderr))
		}
		problems = append(problems, "go.mod or go.sum is not tidy; run `go mod tidy` and commit the result\n"+strings.TrimSpace(tidy.Stdout))
	default:
		return nil, fmt.Errorf("go mod tidy -diff exited %d: %s", tidy.Code, strings.TrimSpace(tidy.Stderr))
	}
	verify, err := Capture(ctx, dir, goBinary, "mod", "verify")
	if err != nil {
		return nil, err
	}
	if verify.Code != 0 {
		problems = append(problems, "go mod verify failed: a downloaded dependency does not match its recorded hash\n"+strings.TrimSpace(verify.Combined()))
	}
	return problems, nil
}
