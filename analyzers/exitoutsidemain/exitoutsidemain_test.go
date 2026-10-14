package exitoutsidemain_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/LongTermSupport/go-qa-ci/analyzers/exitoutsidemain"
)

func TestRedFixtures(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), exitoutsidemain.Analyzer, "red")
}

func TestCleanFixture(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), exitoutsidemain.Analyzer, "clean")
}

func TestIdentifier(t *testing.T) {
	if exitoutsidemain.Analyzer.Name != "gqaci_exit_outside_main" {
		t.Errorf("name %q", exitoutsidemain.Analyzer.Name)
	}
}

func TestLibraryPackageNamedMainFunctionIsNotTheEntryPoint(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), exitoutsidemain.Analyzer, "library")
}
