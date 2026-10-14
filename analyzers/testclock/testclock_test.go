package testclock_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/LongTermSupport/go-qa-ci/analyzers/testclock"
)

func TestRedFixtures(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), testclock.Analyzer, "red")
}

func TestCleanFixture(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), testclock.Analyzer, "clean")
}

func TestIdentifier(t *testing.T) {
	if testclock.Analyzer.Name != "gqaci_test_clock" {
		t.Errorf("name %q", testclock.Analyzer.Name)
	}
}
