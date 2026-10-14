package runners

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/LongTermSupport/go-qa-ci/internal/finding"
)

const (
	deadcodeBinary = "deadcode"
	// DeadcodeRule is the identifier deadcode findings are reported and recorded under.
	DeadcodeRule = "deadcode"
	mainPackage  = "main"
)

type deadPackage struct {
	Path  string `json:"Path"`
	Funcs []struct {
		Name     string `json:"Name"`
		Position struct {
			File string `json:"File"`
			Line int    `json:"Line"`
		} `json:"Position"`
		Generated bool `json:"Generated"`
		Marker    bool `json:"Marker"`
	} `json:"Funcs"`
}

// ParseDeadcode reads `deadcode -json` output. The tool prints null when nothing is dead.
func ParseDeadcode(data []byte) ([]finding.Finding, error) {
	var packages []deadPackage
	if err := json.Unmarshal(data, &packages); err != nil {
		return nil, fmt.Errorf("reading %s output: %w", deadcodeBinary, err)
	}
	var findings []finding.Finding
	for _, pkg := range packages {
		for _, fn := range pkg.Funcs {
			if fn.Generated || fn.Marker {
				continue
			}
			findings = append(findings, finding.Finding{
				Rule:    DeadcodeRule,
				Path:    fn.Position.File,
				Line:    fn.Position.Line,
				Message: fmt.Sprintf("%s.%s is unreachable from every main function and test", pkg.Path, fn.Name),
			})
		}
	}
	return findings, nil
}

// HasMain reports whether the module in dir contains a main package. deadcode starts from main
// functions, so for a library it would call every exported function dead; there the `unused`
// linter is the defence.
func HasMain(ctx context.Context, dir string) (bool, error) {
	out, err := Capture(ctx, dir, goBinary, "list", "-f", "{{.Name}}", "./...")
	if err != nil {
		return false, err
	}
	if out.Code != 0 {
		return false, fmt.Errorf("go list exited %d: %s", out.Code, strings.TrimSpace(out.Stderr))
	}
	for _, name := range strings.Fields(out.Stdout) {
		if name == mainPackage {
			return true, nil
		}
	}
	return false, nil
}

// Deadcode lists the functions of the module in dir that no main function or test reaches.
// applies is false for a module without a main package.
func Deadcode(ctx context.Context, dir string) (findings []finding.Finding, applies bool, err error) {
	hasMain, err := HasMain(ctx, dir)
	if err != nil || !hasMain {
		return nil, false, err
	}
	if locateErr := Locate(deadcodeBinary); locateErr != nil {
		return nil, true, locateErr
	}
	out, err := Capture(ctx, dir, deadcodeBinary, "-test", "-json", "./...")
	if err != nil {
		return nil, true, err
	}
	if out.Code != 0 {
		return nil, true, fmt.Errorf("%s exited %d: %s", deadcodeBinary, out.Code, strings.TrimSpace(out.Stderr))
	}
	findings, err = ParseDeadcode([]byte(out.Stdout))
	return findings, true, err
}
