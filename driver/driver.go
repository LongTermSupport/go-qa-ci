// Package driver is the go/analysis driver a project builds its own analyzers into.
package driver

import (
	"slices"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/unitchecker"

	"github.com/LongTermSupport/go-qa-ci/analyzers/pack"
	"github.com/LongTermSupport/go-qa-ci/analyzers/sensitiveformat"
)

// Main runs the bundled analyzers and the given ones as a go vet tool and never returns. The
// bundled analyzers are always registered, so a project driver cannot switch a Blocking defence
// off by leaving it out; a configured sensitiveformat.New replaces the bundled one, and an
// analyzer of the bundled pack may be passed itself. An analyzer that only takes a bundled name is
// a mistake and panics, as it would otherwise replace the defence with itself.
func Main(extra ...*analysis.Analyzer) {
	unitchecker.Main(withBundled(extra)...)
}

// built reports whether a may stand in for the bundled analyzer: the bundled analyzer itself, or,
// for the one that is configurable, one made by its constructor.
func built(a, bundled *analysis.Analyzer) bool {
	if a == bundled {
		return true
	}
	return bundled == sensitiveformat.Analyzer && sensitiveformat.Built(a)
}

// withBundled returns extra followed by every bundled analyzer that extra does not already replace.
// Only the bundled analyzer's own constructor replaces it; any other analyzer under a bundled name
// panics.
func withBundled(extra []*analysis.Analyzer) []*analysis.Analyzer {
	all := slices.Clone(extra)
	for _, bundled := range pack.All() {
		replaced := false
		for _, a := range extra {
			if a.Name != bundled.Name {
				continue
			}
			if !built(a, bundled) {
				panic("driver: analyzer " + a.Name + " takes the name of a bundled analyzer without being built by its constructor; the bundled defence would be replaced by it")
			}
			replaced = true
		}
		if !replaced {
			all = append(all, bundled)
		}
	}
	return all
}
