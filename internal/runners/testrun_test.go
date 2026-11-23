package runners_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/runners"
)

const passingSource = "package fixture\n\nfunc Double(n int) int { return n * 2 }\n"

func TestTestPassesAGreenModule(t *testing.T) {
	dir := module(t, map[string]string{
		"f.go":      passingSource,
		"f_test.go": "package fixture\n\nimport \"testing\"\n\nfunc TestDouble(t *testing.T) {\n\tif Double(2) != 4 {\n\t\tt.Fatal(\"bad\")\n\t}\n}\n",
	})
	output, passed, err := runners.Test(ctx(), dir, runners.TestPlan{})
	if err != nil || !passed {
		t.Errorf("got %q, %v, %v", output, passed, err)
	}
}

// Defence for the class "a QA runner makes Go redo work whose inputs did not change": a second
// default run over an unchanged module must be answered from Go's test cache, not rerun.
func TestTheDefaultTestRunIsServedFromTheCacheWhenNothingChanged(t *testing.T) {
	dir := module(t, map[string]string{
		"f.go":      passingSource,
		"f_test.go": "package fixture\n\nimport \"testing\"\n\nfunc TestDouble(t *testing.T) {\n\tif Double(2) != 4 {\n\t\tt.Fatal(\"bad\")\n\t}\n}\n",
	})
	for run := range 2 {
		output, passed, err := runners.Test(ctx(), dir, runners.TestPlan{})
		if err != nil || !passed {
			t.Fatalf("run %d: got %q, %v, %v", run, output, passed, err)
		}
		if run == 1 && !strings.Contains(output, "(cached)") {
			t.Errorf("the second run over an unchanged module reran the tests instead of using the test cache:\n%s", output)
		}
	}
}

func TestTestFailsARedModuleAndKeepsTheOutput(t *testing.T) {
	dir := module(t, map[string]string{
		"f.go":      passingSource,
		"f_test.go": "package fixture\n\nimport \"testing\"\n\nfunc TestDouble(t *testing.T) {\n\tt.Fatal(\"deliberate failure\")\n}\n",
	})
	output, passed, err := runners.Test(ctx(), dir, runners.TestPlan{})
	if err != nil || passed || !strings.Contains(output, "deliberate failure") {
		t.Errorf("got %q, %v, %v", output, passed, err)
	}
}

// The opt-out: a forced run reruns every test, shuffled, however recently it passed.
func TestAForcedRunRerunsShuffledTestsThatAreCached(t *testing.T) {
	dir := module(t, runProbe(true, true))
	if output, passed, err := runners.Test(ctx(), dir, runners.TestPlan{}); err == nil && passed {
		t.Fatalf("the probe passed a default run, so it cannot tell a forced one apart:\n%s", output)
	}
	for run := range 2 {
		output, passed, err := runners.Test(ctx(), dir, runners.TestPlan{Forced: true})
		if err != nil || !passed || strings.Contains(output, "(cached)") {
			t.Errorf("forced run %d: got %q, %v, %v", run, output, passed, err)
		}
	}
}

// Each batch runs its own packages, and only a batch that asks for it is built with -race.
func TestEachBatchRunsItsPackagesWithOrWithoutTheRaceDetector(t *testing.T) {
	files := map[string]string{}
	for name, content := range runProbe(true, false) {
		files["raced/"+name] = strings.Replace(content, "package fixture", "package raced", 1)
	}
	for name, content := range runProbe(false, false) {
		files["plain/"+name] = strings.Replace(content, "package fixture", "package plain", 1)
	}
	dir := module(t, files)
	plan := runners.TestPlan{Batches: []runners.Batch{{Packages: []string{"./raced"}, Race: true}, {Packages: []string{"./plain"}}}}
	if output, passed, err := runners.Test(ctx(), dir, plan); err != nil || !passed {
		t.Errorf("got %q, %v, %v", output, passed, err)
	}
	plan.Batches[1].Race = true
	if output, passed, err := runners.Test(ctx(), dir, plan); err != nil || passed || !strings.Contains(output, "race detector on") {
		t.Errorf("a failing second batch must fail the run: got %q, %v, %v", output, passed, err)
	}
}

// runProbe is a package whose test fails unless the race detector is on (race) or off, and the
// run is shuffled (forced) or not. -count cannot be probed: its default is 1 as well.
func runProbe(race, forced bool) map[string]string {
	shuffle := "off"
	if forced {
		shuffle = "on"
	}
	return map[string]string{
		"f.go":        "package fixture\n\nfunc Used() int { return 1 }\n",
		"race_on.go":  "//go:build race\n\npackage fixture\n\nconst raceEnabled = true\n",
		"race_off.go": "//go:build !race\n\npackage fixture\n\nconst raceEnabled = false\n",
		"f_test.go": "package fixture\n\nimport (\n\t\"flag\"\n\t\"testing\"\n)\n\nfunc TestRunsAsAsked(t *testing.T) {\n" +
			"\tif raceEnabled != " + strconv.FormatBool(race) + " {\n\t\tt.Errorf(\"race detector on = %v\", raceEnabled)\n\t}\n" +
			"\tif got := flag.Lookup(\"test.shuffle\").Value.String(); got != \"" + shuffle + "\" {\n" +
			"\t\tt.Errorf(\"-shuffle=%s, want " + shuffle + "\", got)\n\t}\n}\n",
	}
}
