package record

import (
	"fmt"
	"path/filepath"

	"github.com/LongTermSupport/go-qa-ci/internal/finding"
)

// Check loads the record at rel under root and validates it. rel is the path as the project
// configures it (slash-separated, relative to root) and is what findings show. A missing record
// is an empty one; an unreadable or unparseable record is an error.
func Check(root, rel string) (Record, []finding.Finding, error) {
	rec, err := Load(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return Record{}, nil, fmt.Errorf("%s: %w", rel, err)
	}
	return rec, rec.Validate(rel), nil
}
