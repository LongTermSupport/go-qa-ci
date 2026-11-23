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
	profileMode  = 0o600
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
	// Uncovered lists the functions no test reaches, as "file:line: name".
	Uncovered []string
	// Total is the percentage of statements covered.
	Total float64
	// Statements is the number of coverable statements counted. Zero means the measured packages
	// hold no statement at all, so Total (which go then reports as 0.0%) measures nothing.
	Statements int
}

// MeasureCoverage runs the module's tests in dir exactly as the test step does (TestAndMeasure)
// and returns the coverage; tests that did not pass are a TestsFailedError, since nothing was
// measured.
func MeasureCoverage(ctx context.Context, dir string, measured []string, plan TestPlan) (Coverage, error) {
	output, passed, cov, err := TestAndMeasure(ctx, dir, measured, plan)
	if err != nil {
		return Coverage{}, err
	}
	if !passed {
		return Coverage{}, &TestsFailedError{Output: output}
	}
	return cov, nil
}

// TestAndMeasure is the one test run of the gate: the module's tests in dir as the plan says
// (TestPlan: cacheable unless forced), counting statements in atomic mode, which is correct under
// the race detector. The same run supplies the test verdict (passed, with output everything go
// test printed) and the coverage, which is zero and meaningless when the tests did not pass. Each
// batch writes its own profile and the profiles are read together. measured names the packages
// whose statements are counted; none means every package, so a narrowed run can test the
// importers of a change, whose tests may reach it, and count only the changed code.
func TestAndMeasure(ctx context.Context, dir string, measured []string, plan TestPlan) (output string, passed bool, cov Coverage, err error) {
	scratch, err := os.MkdirTemp("", "go-qa-coverage-")
	if err != nil {
		return "", false, Coverage{}, fmt.Errorf("creating a coverage directory: %w", err)
	}
	if len(measured) == 0 {
		measured = []string{AllPackages}
	}
	output, passed, cov, err = testAndMeasure(ctx, dir, scratch, measured, plan)
	if removeErr := os.RemoveAll(scratch); removeErr != nil && err == nil {
		err = fmt.Errorf("removing %s: %w", scratch, removeErr)
	}
	return output, passed, cov, err
}

func testAndMeasure(ctx context.Context, dir, scratch string, measured []string, plan TestPlan) (output string, passed bool, cov Coverage, err error) {
	var combined strings.Builder
	passed = true
	var profiles []string
	for i, b := range plan.batches() {
		profile := strconv.Itoa(i) + "-" + profileName
		cover := []string{"-covermode=atomic", "-coverpkg=" + strings.Join(measured, ","), "-coverprofile=" + filepath.Join(scratch, profile)}
		tests, err := Capture(ctx, dir, goBinary, plan.args(b, cover...)...)
		if err != nil {
			return "", false, Coverage{}, err
		}
		combined.WriteString(tests.Combined())
		passed = passed && tests.Code == 0
		profiles = append(profiles, profile)
	}
	if !passed {
		return combined.String(), false, Coverage{}, nil
	}
	if err = mergeProfiles(scratch, profileName, profiles); err != nil {
		return "", false, Coverage{}, err
	}
	profile := filepath.Join(scratch, profileName)
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
	cov.Statements, err = profileStatements(profile)
	if err != nil {
		return "", false, Coverage{}, err
	}
	return combined.String(), true, cov, nil
}

// mergeProfiles writes the profiles of every batch, named in the scratch directory, into one
// profile there called name: the first mode line, then every block line. A block listed by two
// batches is summed by go tool cover and counted once by CountProfileStatements, as it already is
// within one -coverpkg profile.
func mergeProfiles(scratch, name string, parts []string) (err error) {
	dir, err := os.OpenRoot(scratch)
	if err != nil {
		return fmt.Errorf("opening the coverage directory: %w", err)
	}
	defer func() {
		if closeErr := dir.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("closing the coverage directory: %w", closeErr)
		}
	}()
	var merged strings.Builder
	for i, part := range parts {
		data, err := dir.ReadFile(part)
		if err != nil {
			return fmt.Errorf("reading the coverage profile: %w", err)
		}
		for line := range strings.Lines(string(data)) {
			if i > 0 && strings.HasPrefix(line, "mode:") {
				continue
			}
			merged.WriteString(line)
		}
	}
	if err = dir.WriteFile(name, []byte(merged.String()), profileMode); err != nil {
		return fmt.Errorf("writing the coverage profile: %w", err)
	}
	return nil
}

// profileStatements is the number of coverable statements the profile at path holds.
func profileStatements(path string) (count int, err error) {
	dir, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return 0, fmt.Errorf("opening the coverage directory: %w", err)
	}
	defer func() {
		if closeErr := dir.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("closing the coverage directory: %w", closeErr)
		}
	}()
	data, err := dir.ReadFile(filepath.Base(path))
	if err != nil {
		return 0, fmt.Errorf("reading the coverage profile: %w", err)
	}
	return CountProfileStatements(string(data))
}

// CountProfileStatements is the number of coverable statements a coverage profile holds, each
// block counted once: a profile written with -coverpkg lists a block again for every test binary
// that ran it.
func CountProfileStatements(profile string) (int, error) {
	blocks := map[string]int{}
	for line := range strings.Lines(profile) {
		fields := strings.Fields(line)
		if len(fields) == 0 || strings.HasPrefix(fields[0], "mode:") {
			continue
		}
		if len(fields) != 3 {
			return 0, fmt.Errorf("unreadable coverage profile line %q", strings.TrimSpace(line))
		}
		n, err := strconv.Atoi(fields[1])
		if err != nil || n < 0 {
			return 0, fmt.Errorf("unreadable statement count in coverage profile line %q", strings.TrimSpace(line))
		}
		blocks[fields[0]] = n
	}
	total := 0
	for _, n := range blocks {
		total += n
	}
	return total, nil
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
