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
// off by leaving it out; a configured sensitiveformat.New replaces the bundled one. An analyzer
// that only takes a bundled name is a mistake and panics, as it would otherwise replace the
// defence with itself.
func Main(extra ...*analysis.Analyzer) {
	unitchecker.Main(withBundled(extra)...)
}

// withBundled returns extra followed by every bundled analyzer that extra does not already replace.
// Only the bundled analyzer's own constructor replaces it; any other analyzer under a bundled name
// panics.
func withBundled(extra []*analysis.Analyzer) []*analysis.Analyzer {
	all := slices.Clone(extra)
	for _, bundled := range []struct {
		analyzer *analysis.Analyzer
		built    func(*analysis.Analyzer) bool
	}{{sensitiveformat.Analyzer, sensitiveformat.Built}} {
		replaced := false
		for _, a := range extra {
			if a.Name != bundled.analyzer.Name {
				continue
			}
			if !bundled.built(a) {
				panic("driver: analyzer " + a.Name + " takes the name of a bundled analyzer without being built by its constructor; the bundled defence would be replaced by it")
			}
			replaced = true
		}
		if !replaced {
			all = append(all, bundled.analyzer)
		}
	}
	return all
}
