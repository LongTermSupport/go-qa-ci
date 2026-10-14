package shellexec_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/LongTermSupport/go-qa-ci/analyzers/shellexec"
)

func TestRedFixtures(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), shellexec.Analyzer, "red")
}

func TestCleanFixture(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), shellexec.Analyzer, "clean")
}

func TestIdentifier(t *testing.T) {
	if shellexec.Analyzer.Name != "gqaci_shell_exec" {
		t.Errorf("name %q", shellexec.Analyzer.Name)
	}
}
