package unconditionalskip_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/LongTermSupport/go-qa-ci/analyzers/unconditionalskip"
)

func TestRedFixtures(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), unconditionalskip.Analyzer, "red")
}

func TestCleanFixture(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), unconditionalskip.Analyzer, "clean")
}

func TestIdentifier(t *testing.T) {
	if unconditionalskip.Analyzer.Name != "gqaci_unconditional_skip" {
		t.Errorf("name %q", unconditionalskip.Analyzer.Name)
	}
}
