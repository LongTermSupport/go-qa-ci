package record

import (
	"github.com/LongTermSupport/go-qa-ci/internal/config"
	"github.com/LongTermSupport/go-qa-ci/internal/date"
	"github.com/LongTermSupport/go-qa-ci/internal/finding"
)

// Check runs the record gate without modifying the record: it loads the record at policy.Path
// under root, validates its entries and reviews it against the policy and the ratchet as of today.
// policy.Path is the path as the project configures it (slash-separated, relative to root) and is
// what findings show. A missing record is an empty one; an unreadable or unparseable record is an
// error.
func Check(root string, policy config.RecordPolicy, today date.Date) (Record, []finding.Finding, error) {
	report, err := Gate(root, policy, today, false)
	if err != nil {
		return Record{}, nil, err
	}
	return report.Record, report.Findings, nil
}
