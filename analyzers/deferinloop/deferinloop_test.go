package deferinloop_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/LongTermSupport/go-qa-ci/analyzers/deferinloop"
)

func TestRedFixtures(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), deferinloop.Analyzer, "red")
}

func TestCleanFixture(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), deferinloop.Analyzer, "clean")
}

func TestIdentifier(t *testing.T) {
	if deferinloop.Analyzer.Name != "gqaci_defer_in_loop" {
		t.Errorf("name %q", deferinloop.Analyzer.Name)
	}
}
