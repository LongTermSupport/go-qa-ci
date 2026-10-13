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
	"strings"
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

// Relative returns path as a slash-separated path relative to dir, resolving symlinks on both
// sides; "." is the root itself. A path that does not exist or lies outside dir is an error.
func Relative(dir, path string) (string, error) {
	base, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", fmt.Errorf("root %s: %w", dir, err)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("path %s: %w", path, err)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", fmt.Errorf("path %s: %w", path, err)
	}
	rel, err := filepath.Rel(base, resolved)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%s is outside the project root %s", path, dir)
	}
	return filepath.ToSlash(rel), nil
}

// InScope reports whether path (relative to the root, slash-separated) lies within the scope: an
// empty scope, or one naming the root, covers everything; otherwise path must be one of the
// scope entries or below one of them.
func InScope(path string, scope []string) bool {
	if len(scope) == 0 {
		return true
	}
	for _, entry := range scope {
		if entry == "." || path == entry || strings.HasPrefix(path, entry+"/") {
			return true
		}
	}
	return false
}
