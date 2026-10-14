// Package finding is what a detector reports: a stable identifier, a location and a message.
package finding

import (
	"fmt"

	"github.com/LongTermSupport/go-qa-ci/internal/builtin"
)

// Identifiers of the findings the toolchain itself raises.
const (
	// RecordInvalid marks a malformed or insufficiently justified record exception.
	RecordInvalid = builtin.RecordInvalid
	// RecordStale marks a record exception that licenses nothing the defences report.
	RecordStale = builtin.RecordStale
	// RecordBudget marks a record that is over one of its size budgets.
	RecordBudget = builtin.RecordBudget
	// RecordRatchet marks a record that grew, or had its ceiling raised, without a signed decision.
	RecordRatchet = builtin.RecordRatchet
	// RecordExpired marks a record exception past its review date.
	RecordExpired = builtin.RecordExpired
	// SuppressionUnrecorded marks a suppression with no exception in the project record.
	SuppressionUnrecorded = builtin.SuppressionUnrecorded
	// SuppressionBare marks a suppression that names no rule or gives no reason.
	SuppressionBare = builtin.SuppressionBare
	// SummaryStale marks a generated agent summary region that is out of date.
	SummaryStale = builtin.SummaryStale
	// DocsDangling marks an identifier that resolves to no documentation.
	DocsDangling = builtin.DocsDangling
	// DocsOrphan marks a project page that documents no identifier the project can see.
	DocsOrphan = builtin.DocsOrphan
)

// Finding is one defect a detector reports.
type Finding struct {
	// Rule is the identifier exactly as the defence prints it.
	Rule string
	// Path is the file, relative to the project root.
	Path string
	// Line is 1-based; 0 means the finding has no line.
	Line int
	// Message says what is wrong.
	Message string
}

// String renders the finding compiler-style, with the identifier in the first line.
func (f Finding) String() string {
	location := f.Path
	if f.Line > 0 {
		location = fmt.Sprintf("%s:%d", f.Path, f.Line)
	}
	return fmt.Sprintf("error[%s]: %s\n  --> %s", f.Rule, f.Message, location)
}
