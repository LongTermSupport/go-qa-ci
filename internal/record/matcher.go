package record

import (
	"fmt"
	"strings"

	"github.com/LongTermSupport/go-qa-ci/internal/finding"
	"github.com/LongTermSupport/go-qa-ci/internal/glob"
)

// Matcher answers the questions detectors ask of the record and remembers which exceptions were
// used, so an exception that licenses nothing can be reported as stale.
type Matcher struct {
	record Record
	used   []bool
}

// Matcher returns a fresh matcher over the record; no exception counts as used yet.
func (r Record) Matcher() *Matcher {
	return &Matcher{record: r, used: make([]bool, len(r.Exceptions))}
}

// Suppression returns the index of the first exception that licenses a suppression of rule at
// path (relative to the project root, slash-separated), and marks it used.
func (m *Matcher) Suppression(rule, path string) (int, bool) {
	return m.find(CoversSuppressions, rule, path)
}

// Finding returns the index of the first exception that covers the findings of rule at path, and
// marks it used.
func (m *Matcher) Finding(rule, path string) (int, bool) {
	return m.find(CoversFindings, rule, path)
}

func (m *Matcher) find(covers Covers, rule, path string) (int, bool) {
	for i, e := range m.record.Exceptions {
		if e.EffectiveCovers() != covers || strings.TrimSpace(e.Rule) != strings.TrimSpace(rule) {
			continue
		}
		if ok, err := glob.Match(e.Path, path); err == nil && ok {
			m.used[i] = true
			return i, true
		}
	}
	return 0, false
}

// Stale reports every exception nothing matched, as gqaci.record.stale findings. It is only
// meaningful after every detector that applies the record has run against the same matcher.
func (m *Matcher) Stale(recordPath string) []finding.Finding {
	return m.stale(recordPath, func(Exception) bool { return true })
}

// StaleCovering is Stale limited to the exceptions that cover one kind of thing. A detector that
// alone applies that kind (the suppression guard for suppressions) can judge it without the others.
func (m *Matcher) StaleCovering(covers Covers, recordPath string) []finding.Finding {
	return m.stale(recordPath, func(e Exception) bool { return e.EffectiveCovers() == covers })
}

func (m *Matcher) stale(recordPath string, wanted func(Exception) bool) []finding.Finding {
	var findings []finding.Finding
	for i, e := range m.record.Exceptions {
		if m.used[i] || !wanted(e) {
			continue
		}
		findings = append(findings, finding.Finding{
			Rule: finding.RecordStale,
			Path: recordPath,
			Line: e.Line,
			Message: fmt.Sprintf("exception #%d (rule `%s`, path `%s`) licenses nothing the defences report; remove it",
				i+1, e.Rule, e.Path),
		})
	}
	return findings
}
