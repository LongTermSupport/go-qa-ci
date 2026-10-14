// Package builtin lists the identifiers of the defences implemented inside go-qa itself.
//
// Identifiers are chosen by hand and never derived from paths or type names, so they stay stable
// across refactors. Each one has a bundled page in package ruledocs.
package builtin

import (
	"slices"
	"sort"
)

// Identifiers of the built-in defences.
const (
	// RecordInvalid marks a malformed or insufficiently justified record exception.
	RecordInvalid = "gqaci.record.invalid"
	// RecordStale marks a record exception that licenses nothing the defences report.
	RecordStale = "gqaci.record.stale"
	// RecordBudget marks a record that is over one of its size budgets.
	RecordBudget = "gqaci.record.budget"
	// RecordRatchet marks a record that grew, or had its ceiling raised, without a signed decision.
	RecordRatchet = "gqaci.record.ratchet"
	// RecordExpired marks a record exception past its review date.
	RecordExpired = "gqaci.record.expired"
	// SuppressionUnrecorded marks a suppression with no exception in the project record.
	SuppressionUnrecorded = "gqaci.suppression.unrecorded"
	// SuppressionBare marks a suppression that names no rule, which no record entry can license.
	SuppressionBare = "gqaci.suppression.bare"
	// SummaryStale marks a generated agent summary region that is out of date.
	SummaryStale = "gqaci.summary.stale"
	// DocsDangling marks an identifier that resolves to no documentation.
	DocsDangling = "gqaci.docs.dangling"
	// DocsOrphan marks a project page that documents no identifier the project can see.
	DocsOrphan = "gqaci.docs.orphan"
	// SensitiveFormat marks a value holding a sensitive-named type printed without redaction. It is
	// the one built-in whose identifier is an analyzer name, which is why it uses an underscore.
	SensitiveFormat = "gqaci_sensitive_format"
	// DeferInLoop marks a defer statement inside a loop body.
	DeferInLoop = "gqaci_defer_in_loop"
	// ExecContext marks os/exec.Command used where a context is in scope.
	ExecContext = "gqaci_exec_context"
	// ExitOutsideMain marks a process exit outside the entry point of the program.
	ExitOutsideMain = "gqaci_exit_outside_main"
	// InitSideEffects marks an init function that does input or output or changes the process.
	InitSideEffects = "gqaci_init_side_effects"
	// MapRangeOrder marks output whose order is decided by map iteration.
	MapRangeOrder = "gqaci_map_range_order"
	// ShellExec marks a shell started with -c.
	ShellExec = "gqaci_shell_exec"
	// TestClock marks the wall clock read in a test file.
	TestClock = "gqaci_test_clock"
	// UnconditionalSkip marks a test that skips itself unconditionally.
	UnconditionalSkip = "gqaci_unconditional_skip"
	// RulesUnnamed marks a project ast-grep rule that has no id.
	RulesUnnamed = "gqaci.rules.unnamed"
	// RulesUntested marks a project ast-grep rule without valid and invalid test cases.
	RulesUntested = "gqaci.rules.untested"
)

var descriptions = map[string]string{
	RecordInvalid:         "every project-record exception is well formed, attributed and justified",
	RecordStale:           "every project-record exception still covers something",
	RecordBudget:          "the project record stays within its budgets",
	RecordRatchet:         "the project record grows only by a signed decision",
	RecordExpired:         "no project-record exception is past its review date",
	SuppressionUnrecorded: "every suppression has an exception in the project record",
	SuppressionBare:       "every suppression names the rule it silences",
	SummaryStale:          "the generated agent summary is current",
	DocsDangling:          "every listed identifier resolves to documentation",
	DocsOrphan:            "every project documentation page belongs to an identifier the project can see",
	SensitiveFormat:       "no sensitive value is printed without redaction",
	DeferInLoop:           "no defer sits inside a loop body",
	ExecContext:           "no exec.Command is used where a context is in scope",
	ExitOutsideMain:       "the process ends only from main",
	InitSideEffects:       "no init function does input or output or changes the process",
	MapRangeOrder:         "no output order is decided by map iteration",
	ShellExec:             "no shell is started with -c",
	TestClock:             "no test reads the wall clock",
	UnconditionalSkip:     "no test skips itself unconditionally",
	RulesUnnamed:          "every project ast-grep rule has an id",
	RulesUntested:         "every project ast-grep rule has valid and invalid test cases",
}

// All returns every built-in identifier, sorted, for listing and documentation checks.
func All() []string {
	ids := make([]string, 0, len(descriptions))
	for id := range descriptions {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// analyzers are the identifiers that are analyzer names: they are listed with the analyzers of the
// vet tool, where each is active only when that tool carries it, not with the built-ins of a tool.
var analyzers = []string{
	DeferInLoop, ExecContext, ExitOutsideMain, InitSideEffects, MapRangeOrder,
	SensitiveFormat, ShellExec, TestClock, UnconditionalSkip,
}

// Analyzers returns the identifiers of the bundled analyzers, sorted.
func Analyzers() []string {
	out := append([]string(nil), analyzers...)
	sort.Strings(out)
	return out
}

// IsAnalyzer says whether the identifier is a bundled analyzer.
func IsAnalyzer(id string) bool {
	return slices.Contains(analyzers, id)
}

// Describe says in one line what a built-in identifier guards; it is empty for an unknown one.
func Describe(id string) string {
	return descriptions[id]
}
