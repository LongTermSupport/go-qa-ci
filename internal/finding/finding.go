// Package finding is what a detector reports: a stable identifier, a location and a message.
package finding

import "fmt"

// Identifiers of the findings the toolchain itself raises.
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
