package runners_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/runners"
)

const coverFunc = "example.com/fixture/f.go:3:\tCovered\t100.0%\n" +
	"example.com/fixture/f.go:5:\tNever\t0.0%\n" +
	"total:\t\t\t\t(statements)\t50.0%\n"

func TestParseCoverFuncReadsTheTotalAndTheUncoveredFunctions(t *testing.T) {
	got, err := runners.ParseCoverFunc(coverFunc)
	if err != nil {
		t.Fatal(err)
	}
	want := runners.Coverage{Total: 50, Uncovered: []string{"example.com/fixture/f.go:5: Never"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestParseCoverFuncRejectsReportsItCannotRead(t *testing.T) {
	for name, text := range map[string]string{
		"no total":   "example.com/fixture/f.go:3:\tCovered\t100.0%\n",
		"unreadable": "example.com/fixture/f.go:3:\tCovered\tmany\n",
		"empty":      "",
	} {
		if _, err := runners.ParseCoverFunc(text); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestMeasureCoverageCountsStatementsAcrossPackages(t *testing.T) {
	dir := module(t, map[string]string{
		"f.go":      "package fixture\n\nfunc Used() int { return 1 }\n\nfunc Never() int { return 2 }\n",
		"f_test.go": "package fixture\n\nimport \"testing\"\n\nfunc TestUsed(t *testing.T) {\n\tif Used() != 1 {\n\t\tt.Fatal(\"bad\")\n\t}\n}\n",
	})
	got, err := runners.MeasureCoverage(ctx(), dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Total != 50 || got.Statements != 2 || len(got.Uncovered) != 1 || !strings.HasSuffix(got.Uncovered[0], "Never") {
		t.Errorf("got %+v", got)
	}
}

func TestCountProfileStatementsCountsEachBlockOnce(t *testing.T) {
	profile := "mode: atomic\nm/f.go:3.20,3.30 1 1\nm/f.go:5.21,5.31 2 0\nm/f.go:3.20,3.30 1 0\n"
	if got, err := runners.CountProfileStatements(profile); err != nil || got != 3 {
		t.Errorf("got %d, %v", got, err)
	}
	if got, err := runners.CountProfileStatements("mode: atomic\n"); err != nil || got != 0 {
		t.Errorf("an empty profile: got %d, %v", got, err)
	}
	for _, bad := range []string{"mode: atomic\nm/f.go:3.20,3.30 1\n", "mode: atomic\nm/f.go:3.20,3.30 x 1\n", "mode: atomic\nm/f.go:3.20,3.30 -1 1\n"} {
		if _, err := runners.CountProfileStatements(bad); err == nil {
			t.Errorf("%q was read", bad)
		}
	}
}

// A package that declares itself in a doc file and holds only tests has no statement to cover:
// go reports its total as 0.0%, and Statements says that nothing was counted.
func TestMeasureCoverageCountsNoStatementsInAPackageThatHasNone(t *testing.T) {
	dir := module(t, map[string]string{
		"doc.go":    "// Package fixture holds only tests.\npackage fixture\n",
		"f_test.go": "package fixture\n\nimport \"testing\"\n\nfunc TestNothing(t *testing.T) {\n\tif 1+1 != 2 {\n\t\tt.Fatal(\"bad\")\n\t}\n}\n",
	})
	got, err := runners.MeasureCoverage(ctx(), dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Statements != 0 || got.Total != 0 {
		t.Errorf("got %+v", got)
	}
}

// gateProbe is a module whose test fails unless it was run the way the gate runs tests: race
// detector on, shuffled, uncached (-count=1), and counting statements in atomic mode.
func gateProbe() map[string]string {
	return map[string]string{
		"f.go":        "package fixture\n\nfunc Used() int { return 1 }\n",
		"race_on.go":  "//go:build race\n\npackage fixture\n\nconst raceEnabled = true\n",
		"race_off.go": "//go:build !race\n\npackage fixture\n\nconst raceEnabled = false\n",
		"f_test.go":   "package fixture\n\nimport (\n\t\"flag\"\n\t\"testing\"\n)\n\nfunc TestRunsLikeTheGate(t *testing.T) {\n\tif Used() != 1 {\n\t\tt.Fatal(\"bad\")\n\t}\n\tif testing.CoverMode() != \"atomic\" {\n\t\tt.Errorf(\"cover mode %q\", testing.CoverMode())\n\t}\n\tfor name, want := range map[string]string{\"test.shuffle\": \"on\", \"test.count\": \"1\"} {\n\t\tif got := flag.Lookup(name).Value.String(); got != want {\n\t\t\tt.Errorf(\"-%s=%s, want %s\", name, got, want)\n\t\t}\n\t}\n\tif !raceEnabled {\n\t\tt.Error(\"the race detector is off\")\n\t}\n}\n",
	}
}

func TestMeasureCoverageRunsTheTestsTheWayTheTestStepDoes(t *testing.T) {
	got, err := runners.MeasureCoverage(ctx(), module(t, gateProbe()), nil)
	if err != nil {
		t.Fatalf("the coverage run is not the gate's test run: %v", err)
	}
	if got.Total != 100 {
		t.Errorf("got %+v", got)
	}
}

func TestTestAndMeasureIsOneRunThatFeedsBothVerdicts(t *testing.T) {
	output, passed, cov, err := runners.TestAndMeasure(ctx(), module(t, gateProbe()), nil)
	if err != nil || !passed {
		t.Fatalf("got %q, %v, %v", output, passed, err)
	}
	if cov.Total != 100 {
		t.Errorf("coverage %+v", cov)
	}
}

func TestTestAndMeasureKeepsTheOutputOfFailingTests(t *testing.T) {
	dir := module(t, map[string]string{
		"f.go":      "package fixture\n\nfunc Used() int { return 1 }\n",
		"f_test.go": "package fixture\n\nimport \"testing\"\n\nfunc TestUsed(t *testing.T) {\n\tt.Fatal(\"deliberate failure\")\n}\n",
	})
	output, passed, cov, err := runners.TestAndMeasure(ctx(), dir, nil)
	if err != nil || passed || !strings.Contains(output, "deliberate failure") || cov.Total != 0 {
		t.Errorf("got %q, %v, %+v, %v", output, passed, cov, err)
	}
}

func TestMeasureCoverageReportsFailingTestsAsSuch(t *testing.T) {
	dir := module(t, map[string]string{
		"f.go":      "package fixture\n\nfunc Used() int { return 1 }\n",
		"f_test.go": "package fixture\n\nimport \"testing\"\n\nfunc TestUsed(t *testing.T) {\n\tt.Fatal(\"deliberate failure\")\n}\n",
	})
	_, err := runners.MeasureCoverage(ctx(), dir, nil)
	var failed *runners.TestsFailedError
	if !errors.As(err, &failed) || !strings.Contains(failed.Output, "deliberate failure") {
		t.Errorf("got %v", err)
	}
}
