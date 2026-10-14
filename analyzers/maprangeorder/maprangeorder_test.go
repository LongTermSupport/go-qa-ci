package maprangeorder_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/LongTermSupport/go-qa-ci/analyzers/maprangeorder"
)

func TestRedFixtures(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), maprangeorder.Analyzer, "red")
}

func TestCleanFixture(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), maprangeorder.Analyzer, "clean")
}

func TestIdentifier(t *testing.T) {
	if maprangeorder.Analyzer.Name != "gqaci_map_range_order" {
		t.Errorf("name %q", maprangeorder.Analyzer.Name)
	}
}
