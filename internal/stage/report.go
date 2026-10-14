package stage

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"strings"

	"github.com/LongTermSupport/go-qa-ci/internal/finding"
)

// rawFinding is one entry of the JSON list a stage with output = "findings" prints. The contract
// is closed: an unknown field is an error, so a stage cannot believe it reported something the
// gate ignores.
type rawFinding struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
	Rule    string `json:"rule"`
	Message string `json:"message"`
	Note    string `json:"note"`
}

// Rule is a rule a stage reports.
type Rule struct {
	// ID is the identifier, exactly as the stage prints it.
	ID string `json:"id"`
	// Description says what the rule finds.
	Description string `json:"description"`
	// Doc is the rule's documentation, for a rule whose tool ships it (or a pointer to it) rather
	// than a page the project writes. A project page of the same name takes precedence.
	Doc string `json:"doc"`
}

// decodeStrict decodes exactly one JSON value into v, rejecting unknown fields and trailing text.
func decodeStrict(data string, v any) error {
	dec := json.NewDecoder(strings.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return errors.New("unexpected text after the JSON list")
	}
	return nil
}

// ParseFindings reads the JSON list a stage printed: {"file", "line", "rule", "message", "note"?,
// "column"?} per finding, with file relative to the stage directory dir (or to the root when dir is
// empty); a file may leave dir (../shared/b.ts) as long as it stays inside the project. The findings carry root-relative, slash-separated paths; a note is appended to the
// message in parentheses.
func ParseFindings(data, dir string) ([]finding.Finding, error) {
	var raw []rawFinding
	if err := decodeStrict(data, &raw); err != nil {
		return nil, fmt.Errorf("not a JSON list of findings: %w", err)
	}
	found := make([]finding.Finding, 0, len(raw))
	for i, r := range raw {
		switch {
		case r.Rule == "":
			return nil, fmt.Errorf("finding #%d has no `rule`", i+1)
		case r.Message == "":
			return nil, fmt.Errorf("finding #%d (%s) has no `message`", i+1, r.Rule)
		case r.File == "" || path.IsAbs(r.File) || !filepath.IsLocal(filepath.FromSlash(path.Join(dir, r.File))):
			return nil, fmt.Errorf("finding #%d (%s): `file` %q must be a relative path that stays inside the project", i+1, r.Rule, r.File)
		case r.Line < 0:
			return nil, fmt.Errorf("finding #%d (%s): `line` %d is negative", i+1, r.Rule, r.Line)
		}
		message := r.Message
		if r.Note != "" {
			message = fmt.Sprintf("%s (%s)", r.Message, r.Note)
		}
		found = append(found, finding.Finding{
			Rule:    r.Rule,
			Path:    path.Join(dir, r.File),
			Line:    r.Line,
			Message: message,
		})
	}
	return found, nil
}

// ParseRules reads the JSON list of rules a stage's rules command printed: every rule has an id,
// and no id is listed twice.
func ParseRules(data string) ([]Rule, error) {
	var rules []Rule
	if err := decodeStrict(data, &rules); err != nil {
		return nil, fmt.Errorf("not a JSON list of rules: %w", err)
	}
	seen := make(map[string]bool, len(rules))
	for i, rule := range rules {
		switch {
		case rule.ID == "":
			return nil, fmt.Errorf("rule #%d has no `id`", i+1)
		case seen[rule.ID]:
			return nil, fmt.Errorf("rule `%s` is listed twice", rule.ID)
		}
		seen[rule.ID] = true
	}
	return rules, nil
}
