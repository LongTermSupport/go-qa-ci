package runners

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
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

// MeasureCoverage runs the module's tests in dir exactly as the test step does (TestAndMeasure)
// and returns the coverage; tests that did not pass are a TestsFailedError, since nothing was
// measured.
func MeasureCoverage(ctx context.Context, dir string, packages ...string) (Coverage, error) {
	output, passed, cov, err := TestAndMeasure(ctx, dir, packages...)
	if err != nil {
		return Coverage{}, err
	}
	if !passed {
		return Coverage{}, &TestsFailedError{Output: output}
	}
	return cov, nil
}

// TestAndMeasure is the one test run of the gate: the module's tests in dir with the race
// detector, shuffled and uncached, counting statements across every package (atomic mode, which
// is correct under the race detector). The same run supplies the test verdict (passed, with output
// everything go test printed) and the coverage, which is zero and meaningless when the tests did
// not pass. Given package patterns, it tests and counts only those packages.
func TestAndMeasure(ctx context.Context, dir string, packages ...string) (output string, passed bool, cov Coverage, err error) {
	scratch, err := os.MkdirTemp("", "go-qa-coverage-")
	if err != nil {
		return "", false, Coverage{}, fmt.Errorf("creating a coverage directory: %w", err)
	}
	output, passed, cov, err = testAndMeasure(ctx, dir, filepath.Join(scratch, profileName), orAll(packages))
	if removeErr := os.RemoveAll(scratch); removeErr != nil && err == nil {
		err = fmt.Errorf("removing %s: %w", scratch, removeErr)
	}
	return output, passed, cov, err
}

func testAndMeasure(ctx context.Context, dir, profile string, packages []string) (output string, passed bool, cov Coverage, err error) {
	args := slices.Concat(testFlags, []string{"-covermode=atomic", "-coverpkg=" + strings.Join(packages, ","), "-coverprofile=" + profile}, packages)
	tests, err := Capture(ctx, dir, goBinary, args...)
	if err != nil {
		return "", false, Coverage{}, err
	}
	if tests.Code != 0 {
		return tests.Combined(), false, Coverage{}, nil
	}
	report, err := Capture(ctx, dir, goBinary, "tool", "cover", "-func="+profile)
	if err != nil {
		return "", false, Coverage{}, err
	}
	if report.Code != 0 {
		return "", false, Coverage{}, fmt.Errorf("go tool cover exited %d: %s", report.Code, strings.TrimSpace(report.Stderr))
	}
	cov, err = ParseCoverFunc(report.Stdout)
	if err != nil {
		return "", false, Coverage{}, err
	}
	return tests.Combined(), true, cov, nil
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
