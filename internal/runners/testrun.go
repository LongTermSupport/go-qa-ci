package runners

import (
	"context"
	"slices"
)

// testFlags run every test with the race detector, in random order and uncached.
var testFlags = []string{"test", "-race", "-shuffle=on", "-count=1"}

// AllPackages is the pattern for every package of a module.
const AllPackages = "./..."

// orAll is the package patterns to run, or every package when none are given.
func orAll(patterns []string) []string {
	if len(patterns) == 0 {
		return []string{AllPackages}
	}
	return patterns
}

// Test runs the module's tests in dir. passed is false when a test failed or the code did not
// build; output is then everything go test printed. The tests of the given package patterns run,
// or of every package when there are none.
func Test(ctx context.Context, dir string, packages ...string) (output string, passed bool, err error) {
	out, err := Capture(ctx, dir, goBinary, slices.Concat(testFlags, orAll(packages))...)
	if err != nil {
		return "", false, err
	}
	return out.Combined(), out.Code == 0, nil
}
