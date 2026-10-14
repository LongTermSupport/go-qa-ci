// Package pack is the registry of the analyzers bundled with go-qa. The vet tool, the driver and
// the tests that keep the documentation honest all read this one list, so a rule that is added
// here is registered, documented and counted, and one that is not cannot be silently missing.
package pack

import (
	"golang.org/x/tools/go/analysis"

	"github.com/LongTermSupport/go-qa-ci/analyzers/deferinloop"
	"github.com/LongTermSupport/go-qa-ci/analyzers/execcontext"
	"github.com/LongTermSupport/go-qa-ci/analyzers/exitoutsidemain"
	"github.com/LongTermSupport/go-qa-ci/analyzers/initsideeffects"
	"github.com/LongTermSupport/go-qa-ci/analyzers/maprangeorder"
	"github.com/LongTermSupport/go-qa-ci/analyzers/sensitiveformat"
	"github.com/LongTermSupport/go-qa-ci/analyzers/shellexec"
	"github.com/LongTermSupport/go-qa-ci/analyzers/testclock"
	"github.com/LongTermSupport/go-qa-ci/analyzers/unconditionalskip"
)

// All returns the bundled analyzers, each with its default configuration, sorted by name.
func All() []*analysis.Analyzer {
	return []*analysis.Analyzer{
		deferinloop.Analyzer,
		execcontext.Analyzer,
		exitoutsidemain.Analyzer,
		initsideeffects.Analyzer,
		maprangeorder.Analyzer,
		sensitiveformat.Analyzer,
		shellexec.Analyzer,
		testclock.Analyzer,
		unconditionalskip.Analyzer,
	}
}
