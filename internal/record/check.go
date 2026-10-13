package record

import (
	"fmt"
	"path/filepath"

	"github.com/LongTermSupport/go-qa-ci/internal/config"
	"github.com/LongTermSupport/go-qa-ci/internal/date"
	"github.com/LongTermSupport/go-qa-ci/internal/finding"
)

// Check loads the record at policy.Path under root, validates its entries and reviews it against
// the policy as of today. policy.Path is the path as the project configures it (slash-separated,
// relative to root) and is what findings show. A missing record is an empty one; an unreadable or
// unparseable record is an error.
func Check(root string, policy config.RecordPolicy, today date.Date) (Record, []finding.Finding, error) {
	rec, err := Load(filepath.Join(root, filepath.FromSlash(policy.Path)))
	if err != nil {
		return Record{}, nil, fmt.Errorf("%s: %w", policy.Path, err)
	}
	findings := rec.Validate(policy.Path)
	findings = append(findings, Review(Input{Record: rec, Path: policy.Path, Policy: policy, Today: today})...)
	return rec, findings, nil
}
