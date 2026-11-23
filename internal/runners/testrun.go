package runners

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/LongTermSupport/go-qa-ci/internal/gotest"
)

// AllPackages is the pattern for every package of a module.
const AllPackages = "./..."

// orAll is the package patterns to run, or every package when none are given.
func orAll(patterns []string) []string {
	if len(patterns) == 0 {
		return []string{AllPackages}
	}
	return patterns
}

// Batch is the packages one go test invocation runs, and whether it runs them under the race
// detector.
type Batch struct {
	// Packages are the package patterns; none means every package of the module.
	Packages []string
	// Race builds the tests with the race detector.
	Race bool
}

// TestPlan is how the gate runs a module's tests: one go test invocation per batch, in order.
//
// By default the run is cacheable: Go answers a test whose inputs (its package and every
// dependency, the files it opened inside the module, the environment variables it read, the flags)
// did not change from its test cache instead of running it again. Forced runs every test again,
// uncached and in random order (-shuffle=on -count=1), for a run that wants a fresh verdict, such
// as a nightly one. go treats -shuffle as uncacheable, so random order belongs to a forced run.
type TestPlan struct {
	// Batches are the invocations; none means every package under the race detector.
	Batches []Batch
	// Forced reruns every test, uncached and shuffled.
	Forced bool
	// Report, when set, receives the slowest packages and tests once every batch has run.
	Report io.Writer
}

// forcedFlags rerun every test, in random order. Either one alone already defeats the test cache.
var forcedFlags = []string{"-shuffle=on", "-count=1"}

// cacheDefeated fails a cacheable run whose GOFLAGS (from go env, so `go env -w` counts) carries a
// flag that reruns every test: the run would claim to use the cache and silently not.
func cacheDefeated(ctx context.Context, dir string) error {
	out, err := Capture(ctx, dir, goBinary, "env", "GOFLAGS")
	if err != nil {
		return err
	}
	if out.Code != 0 {
		return fmt.Errorf("go env GOFLAGS exited %d: %s", out.Code, strings.TrimSpace(out.Combined()))
	}
	if flag, found := gotest.RerunFlag(strings.Fields(out.Stdout)); found {
		return fmt.Errorf("GOFLAGS holds %s, which makes go rerun every test the cache already holds; "+
			"remove it from GOFLAGS (and from `go env -w`), and use --no-cache or test.cache = false for a forced run", flag)
	}
	return nil
}

// batches is the plan's invocations, defaulting to every package under the race detector.
func (p TestPlan) batches() []Batch {
	if len(p.Batches) == 0 {
		return []Batch{{Race: true}}
	}
	return p.Batches
}

// args is the go command line of one batch, with extra flags (coverage) before the packages.
func (p TestPlan) args(b Batch, extra ...string) []string {
	args := []string{"test", "-json"}
	if b.Race {
		args = append(args, "-race")
	}
	if p.Forced {
		args = append(args, forcedFlags...)
	}
	return slices.Concat(args, extra, orAll(b.Packages))
}

// Test runs the module's tests in dir as the plan says. passed is false when a test failed or the
// code did not build in any batch; output is everything go test printed, batch after batch.
//
// A failing batch does not stop the next: a single go test invocation reports every failing
// package rather than the first, and the batches keep that, so one run names the broken changed
// package and the importers it broke together. The later batch is the cheap one (no race
// detector, cached where unchanged), so continuing costs little.
func Test(ctx context.Context, dir string, plan TestPlan) (output string, passed bool, err error) {
	if !plan.Forced {
		if err = cacheDefeated(ctx, dir); err != nil {
			return "", false, err
		}
	}
	var combined strings.Builder
	var timings Timings
	passed = true
	for _, b := range plan.batches() {
		out, err := Capture(ctx, dir, goBinary, plan.args(b)...)
		if err != nil {
			return "", false, err
		}
		parsed := ParseTestJSON(out.Stdout)
		combined.WriteString(parsed.Text + out.Stderr)
		timings.add(parsed.Timings)
		passed = passed && out.Code == 0
	}
	if err = plan.report(timings); err != nil {
		return "", false, err
	}
	return combined.String(), passed, nil
}

// report writes the slowest packages and tests of the run to the plan's Report, if it has one.
func (p TestPlan) report(timings Timings) error {
	if p.Report == nil {
		return nil
	}
	if _, err := io.WriteString(p.Report, timings.Slowest(slowestShown)); err != nil {
		return fmt.Errorf("writing the test timings: %w", err)
	}
	return nil
}
