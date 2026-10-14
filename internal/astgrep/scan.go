package astgrep

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Match is one finding of `ast-grep scan --json=compact`.
type Match struct {
	// Rule is the rule identifier.
	Rule string
	// Path is the file as ast-grep printed it.
	Path string
	// Line and Column are 1-based.
	Line, Column int
	// Severity is error, warning, info or hint.
	Severity string
	// Message and Note are the rule's text, trimmed.
	Message, Note string
}

// Blocking reports whether the match fails the detector: only error severity does.
func (m Match) Blocking() bool {
	return m.Severity == "error"
}

type rawMatch struct {
	File     string  `json:"file"`
	RuleID   string  `json:"ruleId"`
	Severity string  `json:"severity"`
	Message  string  `json:"message"`
	Note     *string `json:"note"`
	Range    struct {
		Start struct {
			Line   int `json:"line"`
			Column int `json:"column"`
		} `json:"start"`
	} `json:"range"`
}

// ParseMatches reads the output of `ast-grep scan --json=compact`. Text that is not a JSON list
// of matches means the scan did not run, and is an error.
func ParseMatches(output []byte) ([]Match, error) {
	var raw []rawMatch
	if err := json.Unmarshal(output, &raw); err != nil {
		return nil, fmt.Errorf("ast-grep printed no findings list: %w", err)
	}
	matches := make([]Match, len(raw))
	for i, found := range raw {
		matches[i] = Match{
			Rule:     found.RuleID,
			Path:     found.File,
			Line:     found.Range.Start.Line + 1,
			Column:   found.Range.Start.Column + 1,
			Severity: found.Severity,
			Message:  strings.TrimSpace(found.Message),
		}
		if found.Note != nil {
			matches[i].Note = strings.TrimSpace(*found.Note)
		}
	}
	return matches, nil
}
