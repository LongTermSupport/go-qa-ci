// Package finding is what a detector reports: a stable identifier, a location and a message.
package finding

import "fmt"

// Identifiers of the findings the toolchain itself raises.
const (
	// RecordInvalid marks a malformed or insufficiently justified record exception.
	RecordInvalid = "gqaci.record.invalid"
	// RecordStale marks a record exception that licenses nothing the defences report.
	RecordStale = "gqaci.record.stale"
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
