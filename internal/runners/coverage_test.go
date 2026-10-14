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
	got, err := runners.MeasureCoverage(ctx(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Total != 50 || len(got.Uncovered) != 1 || !strings.HasSuffix(got.Uncovered[0], "Never") {
		t.Errorf("got %+v", got)
	}
}

func TestMeasureCoverageReportsFailingTestsAsSuch(t *testing.T) {
	dir := module(t, map[string]string{
		"f.go":      "package fixture\n\nfunc Used() int { return 1 }\n",
		"f_test.go": "package fixture\n\nimport \"testing\"\n\nfunc TestUsed(t *testing.T) {\n\tt.Fatal(\"deliberate failure\")\n}\n",
	})
	_, err := runners.MeasureCoverage(ctx(), dir)
	var failed *runners.TestsFailedError
	if !errors.As(err, &failed) || !strings.Contains(failed.Output, "deliberate failure") {
		t.Errorf("got %v", err)
	}
}
