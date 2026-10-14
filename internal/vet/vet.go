// Package vet runs go/analysis analyzers through go vet's -vettool protocol and turns what they
// report into findings keyed by the analyzer name.
//
// go vet -json exits 0 whatever the analyzers find, so a caller decides pass or fail from the
// findings returned here, never from the exit status of go vet.
package vet

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/LongTermSupport/go-qa-ci/internal/finding"
	"github.com/LongTermSupport/go-qa-ci/internal/relpath"
)

// EnvVar names the environment variable that, set to "1", makes the go-qa binary behave as a go vet tool.
const EnvVar = "GQACI_VETTOOL"

const (
	markerOn      = "1"
	driverName    = "vetdriver"
	tempPrefix    = "go-qa-vettool-"
	positionParts = 3
)

var errNoGo = errors.New("go not found on PATH: it is a required tool and its absence is never a pass")

// IsVettool reports whether the environment asks this process to behave as a go vet tool.
func IsVettool() bool {
	return os.Getenv(EnvVar) == markerOn
}

// Analyzer is one analyzer a vet tool registers.
type Analyzer struct {
	// Name is the analyzer name: the identifier its findings are printed under.
	Name string
	// Summary is the first line of the analyzer documentation.
	Summary string
}

// Tool is a go vet tool: an executable that speaks the unitchecker protocol.
type Tool struct {
	// Path is the executable.
	Path string
	// Env is added to the environment of every process started with the tool.
	Env []string
}

// Self is the running go-qa binary, asked through the environment to behave as the vet tool.
func Self() (Tool, error) {
	exe, err := os.Executable()
	if err != nil {
		return Tool{}, fmt.Errorf("locating the go-qa executable: %w", err)
	}
	return Tool{Path: exe, Env: []string{EnvVar + "=" + markerOn}}, nil
}

// Build compiles the project driver package pkg (a package pattern relative to moduleDir) into a
// vet tool. cleanup removes it and is always safe to call.
func Build(ctx context.Context, moduleDir, pkg string) (tool Tool, cleanup func(), err error) {
	noop := func() {}
	goBinary, err := exec.LookPath("go")
	if err != nil {
		return Tool{}, noop, errNoGo
	}
	dir, err := os.MkdirTemp("", tempPrefix)
	if err != nil {
		return Tool{}, noop, fmt.Errorf("creating a place for the analyzer driver: %w", err)
	}
	cleanup = func() {
		if removeErr := os.RemoveAll(dir); removeErr != nil {
			fmt.Fprintf(os.Stderr, "go-qa: removing %s: %v\n", dir, removeErr)
		}
	}
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, goBinary, "build", "-o", filepath.Join(dir, driverName), pkg)
	cmd.Dir = moduleDir
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		cleanup()
		return Tool{}, noop, fmt.Errorf("building the analyzer driver %s: %w: %s", pkg, err, strings.TrimSpace(stderr.String()))
	}
	return Tool{Path: filepath.Join(dir, driverName)}, cleanup, nil
}

// Resolve returns the vet tool a project uses: its own driver package when driverPkg is set, and
// otherwise go-qa itself. cleanup removes anything built and is always safe to call.
func Resolve(ctx context.Context, moduleDir, driverPkg string) (tool Tool, cleanup func(), err error) {
	if driverPkg != "" {
		return Build(ctx, moduleDir, driverPkg)
	}
	tool, err = Self()
	return tool, func() {}, err
}

// run executes the tool with args and returns its standard output; a failing exit is an error
// carrying what the tool said on standard error.
func (t Tool) run(ctx context.Context, args ...string) (string, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, t.Path, args...)
	cmd.Env = append(os.Environ(), t.Env...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s %s: %w: %s", filepath.Base(t.Path), strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// vetFlag is one entry of the JSON a vet tool prints for -flags.
type vetFlag struct {
	Name  string
	Usage string
}

// Analyzers lists the analyzers the tool registers, sorted by name. An analyzer is registered
// exactly when the tool offers an enabling flag of its name, which is how unitchecker declares one.
func (t Tool) Analyzers(ctx context.Context) ([]Analyzer, error) {
	out, err := t.run(ctx, "-flags")
	if err != nil {
		return nil, err
	}
	var flags []vetFlag
	if err := json.Unmarshal([]byte(out), &flags); err != nil {
		return nil, fmt.Errorf("reading the analyzer list of %s: %w", filepath.Base(t.Path), err)
	}
	var analyzers []Analyzer
	for _, f := range flags {
		if f.Usage != fmt.Sprintf("enable %q analysis", f.Name) {
			continue
		}
		help, err := t.Help(ctx, f.Name)
		if err != nil {
			return nil, err
		}
		firstLine := strings.SplitN(help, "\n", 2)[0]
		analyzers = append(analyzers, Analyzer{Name: f.Name, Summary: strings.TrimPrefix(firstLine, f.Name+": ")})
	}
	sort.Slice(analyzers, func(i, j int) bool { return analyzers[i].Name < analyzers[j].Name })
	return analyzers, nil
}

// Help returns the documentation of one analyzer, as the tool prints it.
func (t Tool) Help(ctx context.Context, name string) (string, error) {
	return t.run(ctx, "help", name)
}

// Vet runs the tool over every package of the module in dir and returns the findings, with paths
// relative to dir. Code that does not compile is an error: nothing was analysed.
func (t Tool) Vet(ctx context.Context, dir string) ([]finding.Finding, error) {
	goBinary, err := exec.LookPath("go")
	if err != nil {
		return nil, errNoGo
	}
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, goBinary, "vet", "-vettool="+t.Path, "-json", "./...")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), t.Env...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("go vet -json in %s: %w: %s", dir, err, strings.TrimSpace(stderr.String()))
	}
	return Parse(stdout.Bytes(), dir)
}

// diagnostic is one finding in go vet's JSON output.
type diagnostic struct {
	Posn    string
	Message string
}

// Parse reads the stream of JSON objects go vet -json prints, {package: {analyzer: findings}},
// into findings sorted by path and line, with paths relative to dir. A finding two package
// variants both report (a package and its test variant) is returned once. An analyzer that
// reports an error instead of findings, and a position it cannot place inside dir, are errors.
func Parse(data []byte, dir string) ([]finding.Finding, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	seen := map[finding.Finding]bool{}
	var findings []finding.Finding
	for {
		var packages map[string]map[string]json.RawMessage
		err := decoder.Decode(&packages)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading go vet output: %w", err)
		}
		for pkg, analyzers := range packages {
			for name, raw := range analyzers {
				diagnostics, err := diagnosticsOf(raw)
				if err != nil {
					return nil, fmt.Errorf("analyzer %s in %s: %w", name, pkg, err)
				}
				for _, d := range diagnostics {
					f, err := toFinding(name, d, dir)
					if err != nil {
						return nil, err
					}
					if !seen[f] {
						seen[f] = true
						findings = append(findings, f)
					}
				}
			}
		}
	}
	sort.Slice(findings, func(i, j int) bool { return before(findings[i], findings[j]) })
	return findings, nil
}

func before(a, b finding.Finding) bool {
	switch {
	case a.Path != b.Path:
		return a.Path < b.Path
	case a.Line != b.Line:
		return a.Line < b.Line
	case a.Rule != b.Rule:
		return a.Rule < b.Rule
	default:
		return a.Message < b.Message
	}
}

// diagnosticsOf reads an analyzer's entry: the list of findings, or an object carrying the error
// the analyzer failed with.
func diagnosticsOf(raw json.RawMessage) ([]diagnostic, error) {
	var diagnostics []diagnostic
	if err := json.Unmarshal(raw, &diagnostics); err == nil {
		return diagnostics, nil
	}
	var failure struct{ Error string }
	if err := json.Unmarshal(raw, &failure); err != nil || failure.Error == "" {
		return nil, fmt.Errorf("unrecognised entry %s", raw)
	}
	return nil, errors.New(failure.Error)
}

func toFinding(rule string, d diagnostic, dir string) (finding.Finding, error) {
	parts := strings.Split(d.Posn, ":")
	if len(parts) < positionParts {
		return finding.Finding{}, fmt.Errorf("analyzer %s reported a position without file:line:column: %q", rule, d.Posn)
	}
	line, err := strconv.Atoi(parts[len(parts)-2])
	if err != nil {
		return finding.Finding{}, fmt.Errorf("analyzer %s reported a position with a non-numeric line: %q", rule, d.Posn)
	}
	if _, err := strconv.Atoi(parts[len(parts)-1]); err != nil {
		return finding.Finding{}, fmt.Errorf("analyzer %s reported a position with a non-numeric column: %q", rule, d.Posn)
	}
	file := strings.Join(parts[:len(parts)-2], ":")
	rel, ok := relpath.Within(dir, file)
	if !ok {
		return finding.Finding{}, fmt.Errorf("analyzer %s reported %s outside the module %s", rule, file, dir)
	}
	return finding.Finding{Rule: rule, Path: rel, Line: line, Message: d.Message}, nil
}

// WithAnalyzers resolves the project's vet tool (see Resolve), lists its analyzers and runs fn
// with both; anything built is removed afterwards.
func WithAnalyzers(ctx context.Context, moduleDir, driverPkg string, fn func(Tool, []Analyzer) error) error {
	tool, cleanup, err := Resolve(ctx, moduleDir, driverPkg)
	if err != nil {
		return fmt.Errorf("resolving the analyzer driver: %w", err)
	}
	defer cleanup()
	analyzers, err := tool.Analyzers(ctx)
	if err != nil {
		return fmt.Errorf("listing the analyzers: %w", err)
	}
	return fn(tool, analyzers)
}
