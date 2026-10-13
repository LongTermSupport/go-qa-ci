// Package ruledocs holds the pages bundled with go-qa, one markdown file per identifier, embedded
// in the binary so that every identifier go-qa prints resolves offline (TOOLING-SPEC 4.2).
package ruledocs

import (
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

//go:embed rules/*.md
var pages embed.FS

const (
	dir    = "rules"
	suffix = ".md"
)

// Page returns the bundled page for the identifier exactly as printed.
func Page(id string) (string, bool) {
	if id == "" || strings.ContainsAny(id, `/\`) {
		return "", false
	}
	text, err := pages.ReadFile(dir + "/" + id + suffix)
	if err != nil {
		return "", false
	}
	return string(text), true
}

// IDs returns the identifier of every bundled page, sorted.
func IDs() ([]string, error) {
	entries, err := pages.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading the embedded pages: %w", err)
	}
	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		ids = append(ids, strings.TrimSuffix(entry.Name(), suffix))
	}
	sort.Strings(ids)
	return ids, nil
}

// FS is the embedded tree, whose pages are under rules/, for the release audit.
func FS() fs.FS {
	return pages
}
