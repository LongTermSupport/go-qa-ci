// Package driver is the go/analysis driver a project builds its own analyzers into.
package driver

import (
	"slices"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/unitchecker"

	"github.com/LongTermSupport/go-qa-ci/analyzers/sensitiveformat"
)

// Main runs the bundled analyzers and the given ones as a go vet tool and never returns. The
// bundled analyzers are always registered, so a project driver cannot switch a Blocking defence
// off by leaving it out; an analyzer passed under a bundled name (a configured
// sensitiveformat.New, for example) replaces the bundled one.
func Main(extra ...*analysis.Analyzer) {
	unitchecker.Main(withBundled(extra)...)
}

// withBundled returns extra followed by every bundled analyzer that extra does not already name.
func withBundled(extra []*analysis.Analyzer) []*analysis.Analyzer {
	all := slices.Clone(extra)
	for _, bundled := range []*analysis.Analyzer{sensitiveformat.Analyzer} {
		named := slices.ContainsFunc(extra, func(a *analysis.Analyzer) bool { return a.Name == bundled.Name })
		if !named {
			all = append(all, bundled)
		}
	}
	return all
}
