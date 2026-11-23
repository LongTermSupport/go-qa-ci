package runners

import (
	"context"
	"slices"
	"strings"
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
}

// forcedFlags rerun every test, in random order. Either one alone already defeats the test cache.
var forcedFlags = []string{"-shuffle=on", "-count=1"}

// batches is the plan's invocations, defaulting to every package under the race detector.
func (p TestPlan) batches() []Batch {
	if len(p.Batches) == 0 {
		return []Batch{{Race: true}}
	}
	return p.Batches
}

// args is the go command line of one batch, with extra flags (coverage) before the packages.
func (p TestPlan) args(b Batch, extra ...string) []string {
	args := []string{"test"}
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
func Test(ctx context.Context, dir string, plan TestPlan) (output string, passed bool, err error) {
	var combined strings.Builder
	passed = true
	for _, b := range plan.batches() {
		out, err := Capture(ctx, dir, goBinary, plan.args(b)...)
		if err != nil {
			return "", false, err
		}
		combined.WriteString(out.Combined())
		passed = passed && out.Code == 0
	}
	return combined.String(), passed, nil
}
