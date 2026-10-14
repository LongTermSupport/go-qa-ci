package suppression

import (
	"fmt"

	"github.com/LongTermSupport/go-qa-ci/internal/builtin"
	"github.com/LongTermSupport/go-qa-ci/internal/finding"
	"github.com/LongTermSupport/go-qa-ci/internal/record"
)

// Check turns a scan into findings, in file order. An unparseable file is reported because what it
// silences is unknown. A bare suppression is always reported: no record entry can license it. Any
// other suppression is reported unless the matcher's record licenses its rule at its path; the
// matcher remembers which exceptions were used.
func Check(result Result, matcher *record.Matcher) []finding.Finding {
	var findings []finding.Finding
	for _, bad := range result.Unparseable {
		findings = append(findings, finding.Finding{
			Rule:    builtin.SuppressionUnrecorded,
			Path:    bad.Path,
			Message: fmt.Sprintf("cannot parse (%s); what it silences cannot be verified", bad.Message),
		})
	}
	for _, s := range result.Suppressions {
		if s.Bare != "" {
			findings = append(findings, finding.Finding{
				Rule:    builtin.SuppressionBare,
				Path:    s.Path,
				Line:    s.Line,
				Message: fmt.Sprintf("%s: %s; no record entry can license it", s.Route, s.Bare),
			})
		}
		if s.Rule == "" {
			continue
		}
		if _, licensed := matcher.Suppression(s.Rule, s.Path); licensed {
			continue
		}
		findings = append(findings, finding.Finding{
			Rule:    builtin.SuppressionUnrecorded,
			Path:    s.Path,
			Line:    s.Line,
			Message: fmt.Sprintf("%s silences `%s` and no exception in the project record covers it; fix the code, or record the exception with rule `%s` and path `%s`", s.Route, s.Rule, s.Rule, s.Path),
		})
	}
	return findings
}
