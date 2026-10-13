package record

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/LongTermSupport/go-qa-ci/internal/finding"
	"github.com/LongTermSupport/go-qa-ci/internal/glob"
)

// MinJustificationChars is the shortest justification that can name a hazard and a scope.
const MinJustificationChars = 25

// generic lists whole-text justifications that could be pasted onto any exception unchanged. They
// are compared after lower-casing and trimming punctuation from both ends.
var generic = map[string]bool{
	"todo": true, "tbd": true, "fixme": true, "legacy": true, "legacy code": true,
	"needed": true, "needed for now": true, "required": true, "temporary": true, "temp": true,
	"fix later": true, "later": true, "n/a": true, "na": true, "none": true, "ignore": true,
	"wip": true, "hack": true, "workaround": true, "false positive": true,
	"see above": true, "as above": true,
}

// placeholders are words rejected anywhere in a justification.
var placeholders = map[string]bool{"todo": true, "tbd": true, "fixme": true, "xxx": true}

// JustificationProblem says why a justification is unacceptable, or returns "" when it passes the
// check. The check is mechanical (TOOLING-SPEC 6.2): it rejects an empty text, a whole text from
// the generic list, a placeholder word anywhere, and a text shorter than MinJustificationChars.
// It cannot verify that the sentence is true or that it names the right hazard; that is the
// owner's judgement, and passing this check is not evidence of either.
func JustificationProblem(text string) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return "is empty"
	}
	lowered := strings.ToLower(trimmed)
	bare := strings.TrimFunc(lowered, func(r rune) bool { return !isWordRune(r) })
	if generic[bare] {
		return "is generic; state the hazard accepted and the scope of the exception"
	}
	for _, word := range strings.FieldsFunc(lowered, func(r rune) bool { return !isWordRune(r) }) {
		if placeholders[word] {
			return "contains a placeholder (TODO, TBD, FIXME, XXX)"
		}
	}
	if utf8.RuneCountInString(trimmed) < MinJustificationChars {
		return fmt.Sprintf("is too short to name a hazard and a scope (under %d characters)", MinJustificationChars)
	}
	return ""
}

func isWordRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }

// Validate checks every exception for a rule, a valid path, a policy mark on a glob, an explicit
// decider when one is given, and an acceptable justification. It returns one finding per
// defective exception, listing every problem found in it.
func (r Record) Validate(recordPath string) []finding.Finding {
	var findings []finding.Finding
	for i, e := range r.Exceptions {
		problems := e.problems()
		if len(problems) == 0 {
			continue
		}
		findings = append(findings, finding.Finding{
			Rule:    finding.RecordInvalid,
			Path:    recordPath,
			Line:    e.Line,
			Message: fmt.Sprintf("exception #%d (rule `%s`) %s", i+1, e.Rule, strings.Join(problems, "; ")),
		})
	}
	return findings
}

func (e Exception) problems() []string {
	var problems []string
	if strings.TrimSpace(e.Rule) == "" {
		problems = append(problems, "has no rule")
	}
	switch {
	case strings.TrimSpace(e.Path) == "":
		problems = append(problems, "has no path")
	case glob.Valid(e.Path) != nil:
		problems = append(problems, fmt.Sprintf("has an invalid path glob `%s`", e.Path))
	case glob.IsGlob(e.Path) && !e.Policy:
		problems = append(problems, fmt.Sprintf("path `%s` is a glob; a deliberately broad exception must say so with policy = true", e.Path))
	}
	if e.DecidedBy != nil && strings.TrimSpace(*e.DecidedBy) == "" {
		problems = append(problems, "has an empty decided_by")
	}
	if problem := JustificationProblem(e.Justification); problem != "" {
		problems = append(problems, "justification "+problem)
	}
	return problems
}
