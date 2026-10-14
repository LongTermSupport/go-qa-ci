package execcontext_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/LongTermSupport/go-qa-ci/analyzers/execcontext"
)

func TestRedFixtures(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), execcontext.Analyzer, "red")
}

func TestCleanFixture(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), execcontext.Analyzer, "clean")
}

func TestIdentifier(t *testing.T) {
	if execcontext.Analyzer.Name != "gqaci_exec_context" {
		t.Errorf("name %q", execcontext.Analyzer.Name)
	}
}
