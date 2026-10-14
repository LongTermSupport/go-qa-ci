package vettest_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/LongTermSupport/go-qa-ci/internal/vettest"
)

// The single-rule harness: one analyzer, one fixture package, every expected finding marked with
// a want comment and every clean construct left unmarked (DETECTOR-SPEC 4.2).
func TestAnalyzerAgainstItsFixtures(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), vettest.Analyzer, "a")
}

func TestAnalyzerNameIsTheIdentifier(t *testing.T) {
	if vettest.Analyzer.Name != "gqaci_test" {
		t.Errorf("name %q", vettest.Analyzer.Name)
	}
}
