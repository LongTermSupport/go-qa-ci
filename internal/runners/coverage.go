package runners

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	profileName  = "cover.out"
	totalPrefix  = "total:"
	percentSign  = "%"
	zeroCoverage = 0.0
)

// TestsFailedError is returned by MeasureCoverage when the tests it ran did not pass, so no coverage
// was measured.
type TestsFailedError struct {
	// Output is everything go test printed.
	Output string
}

func (e *TestsFailedError) Error() string {
	return "the tests failed while measuring coverage:\n" + e.Output
}

// Coverage is the statement coverage of a module.
type Coverage struct {
	// Total is the percentage of statements covered.
	Total float64
	// Uncovered lists the functions no test reaches, as "file:line: name".
	Uncovered []string
}

// MeasureCoverage runs the module's tests in dir counting statements across every package
// (atomic mode, so it is correct under the race detector too) and reads the total back.
func MeasureCoverage(ctx context.Context, dir string) (Coverage, error) {
	scratch, err := os.MkdirTemp("", "go-qa-coverage-")
	if err != nil {
		return Coverage{}, fmt.Errorf("creating a coverage directory: %w", err)
	}
	cov, measureErr := measure(ctx, dir, filepath.Join(scratch, profileName))
	if removeErr := os.RemoveAll(scratch); removeErr != nil && measureErr == nil {
		measureErr = fmt.Errorf("removing %s: %w", scratch, removeErr)
	}
	return cov, measureErr
}

func measure(ctx context.Context, dir, profile string) (Coverage, error) {
	tests, err := Capture(ctx, dir, goBinary, "test", "-covermode=atomic", "-coverpkg=./...", "-coverprofile="+profile, "./...")
	if err != nil {
		return Coverage{}, err
	}
	if tests.Code != 0 {
		return Coverage{}, &TestsFailedError{Output: tests.Combined()}
	}
	report, err := Capture(ctx, dir, goBinary, "tool", "cover", "-func="+profile)
	if err != nil {
		return Coverage{}, err
	}
	if report.Code != 0 {
		return Coverage{}, fmt.Errorf("go tool cover exited %d: %s", report.Code, strings.TrimSpace(report.Stderr))
	}
	return ParseCoverFunc(report.Stdout)
}

// ParseCoverFunc reads `go tool cover -func` output: the total line gives the percentage, and a
// function line at 0.0% is listed as uncovered.
func ParseCoverFunc(text string) (Coverage, error) {
	var cov Coverage
	found := false
	scanner := bufio.NewScanner(strings.NewReader(text))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		percent, err := strconv.ParseFloat(strings.TrimSuffix(fields[len(fields)-1], percentSign), 64)
		if err != nil {
			return Coverage{}, fmt.Errorf("unreadable coverage line %q", scanner.Text())
		}
		switch {
		case fields[0] == totalPrefix:
			cov.Total, found = percent, true
		case percent == zeroCoverage && len(fields) == 3:
			cov.Uncovered = append(cov.Uncovered, strings.TrimSuffix(fields[0], ":")+": "+fields[1])
		}
	}
	if err := scanner.Err(); err != nil {
		return Coverage{}, fmt.Errorf("reading the coverage report: %w", err)
	}
	if !found {
		return Coverage{}, fmt.Errorf("the coverage report has no total line")
	}
	return cov, nil
}
