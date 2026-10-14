package record

import (
	"fmt"
	"strings"

	"github.com/LongTermSupport/go-qa-ci/internal/finding"
)

// RaiseWindowDays is how recent a signature must be for a raise of the ceiling to count.
const RaiseWindowDays = 14

// Ceiling is the entry count a record commits to: its ratchet, or its size when it has none.
func Ceiling(r Record) int {
	if r.Ratchet == nil {
		return len(r.Exceptions)
	}
	return r.Ratchet.HighWater
}

// RatchetVerdict is the outcome of reviewing the ratchet.
type RatchetVerdict struct {
	// Findings are the gqaci.record.ratchet violations.
	Findings []finding.Finding
	// LowerTo is set when the record shrank: high_water should be lowered to this count.
	LowerTo *int
}

// ReviewRatchet applies the ratchet, a toolchain default that serves TOOLING-SPEC 6.4: the record may not hold entries without a
// signed ceiling, grow past it, or have the ceiling raised except by a configured decider's fresh
// signature. previous is the record as committed before this change, or nil when no history is
// available, in which case a raise cannot be detected.
func ReviewRatchet(in Input, previous *Record) RatchetVerdict {
	count := len(in.Record.Exceptions)
	ratchet := in.Record.Ratchet
	if ratchet == nil {
		if count == 0 {
			return RatchetVerdict{}
		}
		return RatchetVerdict{Findings: []finding.Finding{in.ratchetFinding(fmt.Sprintf(
			"%d exception(s) but no [ratchet] table; add [ratchet] with high_water = %d, decided_by (a configured decider), decided_on and reason%s",
			count, count, in.Policy.DecidersHint()))}}
	}
	problems := ratchetProblems(in, count, previous)
	var verdict RatchetVerdict
	if len(problems) > 0 {
		verdict.Findings = []finding.Finding{in.ratchetFinding("[ratchet] " + strings.Join(problems, "; "))}
	}
	if count < ratchet.HighWater {
		verdict.LowerTo = &count
	}
	return verdict
}

func ratchetProblems(in Input, count int, previous *Record) []string {
	ratchet := in.Record.Ratchet
	var problems []string
	if !in.Policy.IsDecider(ratchet.DecidedBy) {
		problems = append(problems, fmt.Sprintf("decided_by `%s` is not one of the configured deciders%s", ratchet.DecidedBy, in.Policy.DecidersHint()))
	}
	if problem := JustificationProblem(ratchet.Reason); problem != "" {
		problems = append(problems, "reason "+problem)
	}
	if ratchet.DecidedOn.After(in.Today) {
		problems = append(problems, fmt.Sprintf("decided_on %s is in the future", ratchet.DecidedOn))
	}
	if count > ratchet.HighWater {
		problems = append(problems, fmt.Sprintf(
			"the record grew to %d exceptions, past high_water %d; remove entries, or have a configured decider raise high_water with a fresh decided_on and reason",
			count, ratchet.HighWater))
	}
	if previous != nil {
		if before := Ceiling(*previous); ratchet.HighWater > before && !freshSignature(in, previous) {
			since := ""
			if previous.Ratchet != nil {
				since = fmt.Sprintf(" (%s)", previous.Ratchet.DecidedOn)
			}
			problems = append(problems, fmt.Sprintf(
				"high_water raised from %d to %d without a fresh signature: decided_on must be newer than the previous signature%s and within the last %d days",
				before, ratchet.HighWater, since, RaiseWindowDays))
		}
	}
	return problems
}

// freshSignature reports whether the current ratchet was signed after the previous one and within
// the raise window. Dates are whole days, so a second raise on the day of the previous signature is
// accepted when it is a new decision: another decider or a different reason.
func freshSignature(in Input, previous *Record) bool {
	current := in.Record.Ratchet
	newer := previous.Ratchet == nil || current.DecidedOn.After(previous.Ratchet.DecidedOn) ||
		(current.DecidedOn.Equal(previous.Ratchet.DecidedOn) && (current.DecidedBy != previous.Ratchet.DecidedBy || current.Reason != previous.Ratchet.Reason))
	return newer && !current.DecidedOn.Before(in.Today.AddDays(-RaiseWindowDays))
}

func (in Input) ratchetFinding(message string) finding.Finding {
	return in.finding(finding.RecordRatchet, 0, message)
}
