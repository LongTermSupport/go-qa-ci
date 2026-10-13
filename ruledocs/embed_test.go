package ruledocs_test

import (
	"sort"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/ruledocs"
)

func TestPageRoundTripsEveryID(t *testing.T) {
	t.Parallel()
	ids, err := ruledocs.IDs()
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) == 0 {
		t.Fatal("no bundled pages are embedded")
	}
	if !sort.StringsAreSorted(ids) {
		t.Errorf("IDs not sorted: %v", ids)
	}
	for _, id := range ids {
		text, ok := ruledocs.Page(id)
		if !ok || text == "" {
			t.Errorf("Page(%q) = %q, %v", id, text, ok)
		}
	}
}

func TestPageUnknownAndUnsafeIDs(t *testing.T) {
	t.Parallel()
	for _, id := range []string{"gqaci.no.such", "", "../go.mod", "rules/x", "a\\b"} {
		if text, ok := ruledocs.Page(id); ok {
			t.Errorf("Page(%q) found %q", id, text)
		}
	}
}
