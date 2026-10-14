package driver

import (
	"testing"

	"golang.org/x/tools/go/analysis"

	"github.com/LongTermSupport/go-qa-ci/analyzers/sensitiveformat"
)

func names(analyzers []*analysis.Analyzer) []string {
	out := make([]string, 0, len(analyzers))
	for _, a := range analyzers {
		out = append(out, a.Name)
	}
	return out
}

func TestTheBundledAnalyzerIsRegisteredWhenTheProjectNamesNone(t *testing.T) {
	project := &analysis.Analyzer{Name: "project_rule"}
	got := names(withBundled([]*analysis.Analyzer{project}))
	if len(got) != 2 || got[0] != "project_rule" || got[1] != sensitiveformat.Analyzer.Name {
		t.Errorf("got %v", got)
	}
}

func TestAConfiguredBundledAnalyzerReplacesTheBundledOneInsteadOfDuplicatingIt(t *testing.T) {
	configured := sensitiveformat.New(sensitiveformat.Config{})
	got := withBundled([]*analysis.Analyzer{configured})
	if len(got) != 1 || got[0] != configured {
		t.Errorf("got %v", names(got))
	}
}
