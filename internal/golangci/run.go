package golangci

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/LongTermSupport/go-qa-ci/configs"
	"github.com/LongTermSupport/go-qa-ci/internal/finding"
)

const (
	binary         = "golangci-lint"
	exitIssues     = 1
	allPackages    = "./..."
	baselineName   = "golangci.yml"
	baselineMode   = 0o600
	baselineFolder = "go-qa-golangci-"
)

// projectConfigs are the file names golangci-lint itself discovers.
var projectConfigs = []string{".golangci.yml", ".golangci.yaml", ".golangci.toml", ".golangci.json"}

// Runner runs one golangci-lint binary.
type Runner struct {
	// Binary is the path of the golangci-lint executable.
	Binary string
}

// Locate finds golangci-lint on PATH. A missing binary is an error, never a skipped check.
func Locate() (Runner, error) {
	path, err := exec.LookPath(binary)
	if err != nil {
		return Runner{}, fmt.Errorf("%s not found on PATH: install the pinned version; it is a required tool and its absence is never a pass", binary)
	}
	return Runner{Binary: path}, nil
}

// Config resolves the configuration file for a project. When the project at root has its own
// .golangci.* file the path is empty, so golangci-lint discovers it itself; otherwise the
// embedded strict baseline is written to a private temporary file whose path is returned.
// cleanup removes that file and is always safe to call.
func Config(root string) (path string, cleanup func(), err error) {
	noop := func() {}
	for _, name := range projectConfigs {
		_, statErr := os.Stat(filepath.Join(root, name))
		if statErr == nil {
			return "", noop, nil
		}
		if !errors.Is(statErr, os.ErrNotExist) {
			return "", noop, fmt.Errorf("looking for %s: %w", name, statErr)
		}
	}
	dir, err := os.MkdirTemp("", baselineFolder)
	if err != nil {
		return "", noop, fmt.Errorf("writing the baseline configuration: %w", err)
	}
	cleanup = func() {
		if removeErr := os.RemoveAll(dir); removeErr != nil {
			fmt.Fprintf(os.Stderr, "go-qa: removing %s: %v\n", dir, removeErr)
		}
	}
	file := filepath.Join(dir, baselineName)
	if err := os.WriteFile(file, configs.Golangci(), baselineMode); err != nil {
		cleanup()
		return "", noop, fmt.Errorf("writing the baseline configuration: %w", err)
	}
	return file, cleanup, nil
}

// Lint runs every enabled linter over the module in dir and returns the findings, with paths
// relative to dir.
func (r Runner) Lint(ctx context.Context, dir, configPath string) ([]finding.Finding, error) {
	// Parallel runs are allowed: without the flag a second golangci-lint (an editor, another
	// go-qa run) makes this one wait for a global lock and then fail with exit 3.
	args := []string{"run", "--allow-parallel-runners", "--show-stats=false", "--output.json.path=stdout", "--output.text.path=", "--path-mode=abs"}
	args = append(args, configArgs(configPath)...)
	args = append(args, allPackages)
	stdout, stderr, code, err := r.run(ctx, dir, args)
	if err != nil {
		return nil, err
	}
	if code != 0 && code != exitIssues {
		return nil, fmt.Errorf("%s run exited %d: %s", binary, code, strings.TrimSpace(stderr))
	}
	findings, err := Parse([]byte(stdout))
	if err != nil {
		return nil, err
	}
	if err := relativize(findings, dir); err != nil {
		return nil, err
	}
	if code == exitIssues && len(findings) == 0 {
		return nil, fmt.Errorf("%s run reported failure without any issue: %s", binary, strings.TrimSpace(stderr))
	}
	return findings, nil
}

// Format applies the configured formatters to paths (the whole module when empty). With check set
// nothing is modified and the returned diff is what formatting would change.
func (r Runner) Format(ctx context.Context, dir, configPath string, paths []string, check bool) (diff string, err error) {
	args := []string{"fmt"}
	if check {
		args = append(args, "--diff")
	}
	args = append(args, configArgs(configPath)...)
	if len(paths) == 0 {
		paths = []string{allPackages}
	}
	args = append(args, paths...)
	stdout, stderr, code, err := r.run(ctx, dir, args)
	if err != nil {
		return "", err
	}
	switch {
	case code == 0:
		return "", nil
	case check && code == exitIssues && strings.TrimSpace(stdout) != "":
		return stdout, nil
	default:
		return "", fmt.Errorf("%s fmt exited %d: %s", binary, code, strings.TrimSpace(stderr))
	}
}

// relativize rewrites the absolute paths golangci-lint reports to slash-separated paths relative to
// dir. Absolute output is requested because the default base is the configuration file's
// directory, which for the temporary baseline is not the module.
func relativize(findings []finding.Finding, dir string) error {
	bases := []string{dir}
	if resolved, err := filepath.EvalSymlinks(dir); err == nil && resolved != dir {
		bases = append(bases, resolved)
	}
	for i, f := range findings {
		if !filepath.IsAbs(f.Path) {
			continue
		}
		rel, ok := within(bases, f.Path)
		if !ok {
			return fmt.Errorf("%s reported %s outside the module %s", binary, f.Path, dir)
		}
		findings[i].Path = rel
	}
	return nil
}

func within(bases []string, path string) (string, bool) {
	for _, base := range bases {
		rel, err := filepath.Rel(base, path)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return filepath.ToSlash(rel), true
		}
	}
	return "", false
}

func configArgs(configPath string) []string {
	if configPath == "" {
		return nil
	}
	return []string{"--config", configPath}
}

// run executes the binary in dir and returns its output and exit code; err is set only when the
// process could not be run at all.
func (r Runner) run(ctx context.Context, dir string, args []string) (stdout, stderr string, code int, err error) {
	var out, errOut bytes.Buffer
	cmd := exec.CommandContext(ctx, r.Binary, args...)
	cmd.Dir = dir
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	runErr := cmd.Run()
	var exitErr *exec.ExitError
	switch {
	case runErr == nil:
		return out.String(), errOut.String(), 0, nil
	case errors.As(runErr, &exitErr):
		return out.String(), errOut.String(), exitErr.ExitCode(), nil
	default:
		return "", "", 0, fmt.Errorf("running %s: %w", binary, runErr)
	}
}
