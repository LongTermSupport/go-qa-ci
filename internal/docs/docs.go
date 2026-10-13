// Package docs resolves identifiers to remediation pages offline (TOOLING-SPEC 4.2) and audits
// the pages (TOOLING-SPEC 8.1).
//
// Resolution order: a project page in the documentation directory, then a page bundled with
// go-qa. Nothing is fetched from the network. Sources that depend on tools not built yet (the
// golangci-lint and staticcheck catalogues, analyzer help, project stages) are appended to this
// order by the milestones that add them; until then their identifiers are dangling here, which
// is a recorded gap (docs/defences.md).
package docs

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"slices"
	"strings"

	"github.com/LongTermSupport/go-qa-ci/ruledocs"
)

// Source says where a page came from.
type Source int

// The sources, in resolution order.
const (
	// Project is a page in the project documentation directory.
	Project Source = iota
	// Bundled is a page shipped inside go-qa.
	Bundled
)

func (s Source) String() string {
	if s == Project {
		return "project"
	}
	return "bundled"
}

// Doc is a resolved page.
type Doc struct {
	// Source is where the page came from.
	Source Source
	// Path is the project page's path relative to the root; empty for a bundled page.
	Path string
	// Text is the markdown.
	Text string
}

// Resolver resolves identifiers for one project.
type Resolver struct {
	// Root is the project root.
	Root string
	// DocsDir holds the project pages, relative to Root with forward slashes.
	DocsDir string
}

const pageSuffix = ".md"

// FileName is the project page file name for an identifier.
func FileName(id string) string {
	return id + pageSuffix
}

// Key reduces a name to the form pages and identifiers are compared in: lower case, with `-`
// read as `_`, so a page named for a lint-style spelling still documents the identifier.
func Key(name string) string {
	return strings.ReplaceAll(strings.ToLower(name), "-", "_")
}

// usable reports whether id can name a page: non-empty and free of path separators and dot
// segments, so that it can never address a file outside the documentation directory.
func usable(id string) bool {
	return id != "" && id != "." && id != ".." && !strings.ContainsAny(id, `/\`)
}

// Resolve returns the page for id. found is false when the identifier is dangling; an error means
// a page exists but cannot be read, which never falls through to a later source.
func (r Resolver) Resolve(id string) (doc Doc, found bool, err error) {
	id = strings.TrimSpace(id)
	if !usable(id) {
		return Doc{}, false, nil
	}
	doc, found, err = r.project(id)
	if err != nil || found {
		return doc, found, err
	}
	if text, ok := ruledocs.Page(id); ok {
		return Doc{Source: Bundled, Text: text}, true, nil
	}
	return Doc{}, false, nil
}

// project looks for the page under its exact name, then under another spelling of it.
func (r Resolver) project(id string) (Doc, bool, error) {
	root, err := os.OpenRoot(r.Root)
	if err != nil {
		return Doc{}, false, fmt.Errorf("open project root: %w", err)
	}
	doc, found, readErr := r.projectIn(root, id)
	if closeErr := root.Close(); readErr == nil && closeErr != nil {
		return Doc{}, false, fmt.Errorf("close project root: %w", closeErr)
	}
	return doc, found, readErr
}

func (r Resolver) projectIn(root *os.Root, id string) (Doc, bool, error) {
	stems, err := pageStems(root, r.DocsDir)
	if err != nil {
		return Doc{}, false, err
	}
	// The exact name first, then any other spelling of it, in sorted order.
	names := []string{FileName(id)}
	for _, stem := range stems {
		if stem != id && Key(stem) == Key(id) {
			names = append(names, FileName(stem))
		}
	}
	for _, name := range names {
		rel := path.Join(r.DocsDir, name)
		data, readErr := root.ReadFile(rel)
		if errors.Is(readErr, fs.ErrNotExist) {
			continue
		}
		if readErr != nil {
			return Doc{}, false, fmt.Errorf("read project page %s: %w", rel, readErr)
		}
		return Doc{Source: Project, Path: rel, Text: string(data)}, true, nil
	}
	return Doc{}, false, nil
}

// pageStems lists, sorted, the names without suffix of the regular .md files in the directory; a
// missing directory holds none.
func pageStems(root *os.Root, docsDir string) ([]string, error) {
	dir, err := root.Open(docsDir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", docsDir, err)
	}
	entries, readErr := dir.ReadDir(-1)
	if closeErr := dir.Close(); readErr == nil && closeErr != nil {
		return nil, fmt.Errorf("close %s: %w", docsDir, closeErr)
	}
	if readErr != nil {
		return nil, fmt.Errorf("read %s: %w", docsDir, readErr)
	}
	var stems []string
	for _, entry := range entries {
		if name := entry.Name(); strings.HasSuffix(name, pageSuffix) && entry.Type().IsRegular() {
			stems = append(stems, strings.TrimSuffix(name, pageSuffix))
		}
	}
	return sortedStrings(stems), nil
}

func sortedStrings(values []string) []string {
	slices.Sort(values)
	return slices.Compact(values)
}
