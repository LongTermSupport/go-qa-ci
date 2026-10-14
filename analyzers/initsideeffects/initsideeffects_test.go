package initsideeffects_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/LongTermSupport/go-qa-ci/analyzers/initsideeffects"
)

func TestRedFixtures(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), initsideeffects.Analyzer, "red")
}

func TestCleanFixture(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), initsideeffects.Analyzer, "clean")
}

func TestIdentifier(t *testing.T) {
	if initsideeffects.Analyzer.Name != "gqaci_init_side_effects" {
		t.Errorf("name %q", initsideeffects.Analyzer.Name)
	}
}
