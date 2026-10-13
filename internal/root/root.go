// Package root finds the directory go-qa treats as the project root.
package root

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Discover returns the root for dir: the directory of the go.work file when a workspace is active,
// otherwise the directory of the enclosing go.mod.
func Discover(ctx context.Context, dir string) (string, error) {
	cmd := exec.CommandContext(ctx, "go", "env", "-json", "GOWORK", "GOMOD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("go env in %s: %w", dir, err)
	}
	var env struct{ GOWORK, GOMOD string }
	if err := json.Unmarshal(out, &env); err != nil {
		return "", fmt.Errorf("go env output: %w", err)
	}
	marker := env.GOWORK
	if marker == "" || marker == "off" {
		marker = env.GOMOD
	}
	if marker == "" || marker == os.DevNull {
		return "", errors.New("not inside a Go module or workspace: run from one, or pass --root")
	}
	return filepath.Dir(marker), nil
}
