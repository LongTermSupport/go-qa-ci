package driver

import (
	"slices"
	"testing"

	"golang.org/x/tools/go/analysis"

	"github.com/LongTermSupport/go-qa-ci/analyzers/execcontext"
	"github.com/LongTermSupport/go-qa-ci/analyzers/pack"
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
	want := append([]string{"project_rule"}, names(pack.All())...)
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestAConfiguredBundledAnalyzerReplacesTheBundledOneInsteadOfDuplicatingIt(t *testing.T) {
	configured := sensitiveformat.New(sensitiveformat.Config{Patterns: []string{"pin"}})
	got := withBundled([]*analysis.Analyzer{configured})
	if got[0] != configured || len(got) != len(pack.All()) || slices.Contains(got[1:], sensitiveformat.Analyzer) {
		t.Errorf("got %v", names(got))
	}
}

func TestAnAnalyzerThatOnlyReusesTheBundledNameIsRefused(t *testing.T) {
	impostor := &analysis.Analyzer{Name: sensitiveformat.Name, Run: func(*analysis.Pass) (any, error) { return struct{}{}, nil }}
	defer func() {
		if recover() == nil {
			t.Error("a no-op analyzer under the bundled name replaced the bundled defence")
		}
	}()
	withBundled([]*analysis.Analyzer{impostor})
}

func TestTheBundledAnalyzerItselfMayBePassedWithoutDuplicating(t *testing.T) {
	got := withBundled([]*analysis.Analyzer{execcontext.Analyzer})
	if len(got) != len(pack.All()) {
		t.Errorf("got %v", names(got))
	}
}

func TestEveryBundledAnalyzerIsGuardedAgainstAnImpostor(t *testing.T) {
	for _, bundled := range pack.All() {
		impostor := &analysis.Analyzer{Name: bundled.Name, Run: func(*analysis.Pass) (any, error) { return struct{}{}, nil }}
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("a no-op analyzer under the name %s replaced the bundled defence", bundled.Name)
				}
			}()
			withBundled([]*analysis.Analyzer{impostor})
		}()
	}
}
