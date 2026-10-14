// Package pack_test keeps the bundled analyzer pack, its identifiers, its pages and the counts the
// documentation states in step: the registry is the one source and the text is checked against it.
package pack_test

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/analyzers/pack"
	"github.com/LongTermSupport/go-qa-ci/internal/builtin"
	"github.com/LongTermSupport/go-qa-ci/ruledocs"
)

func names() []string {
	var out []string
	for _, a := range pack.All() {
		out = append(out, a.Name)
	}
	return out
}

func read(t *testing.T, path ...string) string {
	t.Helper()
	text, err := os.ReadFile(filepath.Join(append([]string{"..", ".."}, path...)...))
	if err != nil {
		t.Fatal(err)
	}
	return string(text)
}

func TestTheRegistryIsSortedUniqueAndMatchesTheBuiltinIdentifiers(t *testing.T) {
	got := names()
	if !slices.IsSorted(got) || len(slices.Compact(slices.Clone(got))) != len(got) {
		t.Errorf("the registry is not sorted and unique: %v", got)
	}
	if want := builtin.Analyzers(); !slices.Equal(got, want) {
		t.Errorf("registry %v differs from builtin.Analyzers %v", got, want)
	}
}

func TestEveryAnalyzerHasADocStartingWithItsNameAndAPage(t *testing.T) {
	for _, a := range pack.All() {
		if !strings.HasPrefix(a.Doc, a.Name+" ") {
			t.Errorf("%s: the first line of the Doc must start with the name", a.Name)
		}
		if builtin.Describe(a.Name) == "" {
			t.Errorf("%s: no description in internal/builtin", a.Name)
		}
		page, ok := ruledocs.Page(a.Name)
		if !ok || !strings.Contains(page, "\n## Suppression\n") || !strings.Contains(page, "\n## How to fix correctly\n") {
			t.Errorf("%s: the bundled page is missing, or lacks its fix and suppression sections", a.Name)
		}
	}
}

func TestTheDocumentationStatesTheRealCount(t *testing.T) {
	count := strconv.Itoa(len(pack.All()))
	counted := regexp.MustCompile(`\b(\d+) bundled analyzers`)
	for _, file := range [][]string{{"README.md"}, {"docs", "analyzers.md"}, {"docs", "defences.md"}} {
		found := counted.FindAllStringSubmatch(read(t, file...), -1)
		if len(found) == 0 {
			t.Errorf("%s states no count of bundled analyzers", filepath.Join(file...))
		}
		for _, m := range found {
			if m[1] != count {
				t.Errorf("%s says %s bundled analyzers, the registry holds %s", filepath.Join(file...), m[1], count)
			}
		}
	}
	table := regexp.MustCompile("(?m)^\\| `(gqaci_[a-z_]+)` ")
	var listed []string
	for _, m := range table.FindAllStringSubmatch(read(t, "docs", "analyzers.md"), -1) {
		listed = append(listed, m[1])
	}
	if !slices.Equal(listed, names()) {
		t.Errorf("docs/analyzers.md lists %v, the registry holds %v", listed, names())
	}
}
