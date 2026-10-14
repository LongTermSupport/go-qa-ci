// Package driver is the go/analysis driver a project builds its own analyzers into.
package driver

import (
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/unitchecker"
)

// Main runs the analyzers as a go vet tool and never returns.
func Main(extra ...*analysis.Analyzer) {
	unitchecker.Main(extra...)
}
