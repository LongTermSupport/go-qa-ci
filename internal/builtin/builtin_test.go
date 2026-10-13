package builtin_test

import (
	"sort"
	"strings"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/builtin"
)

func TestEveryBuiltinIsNamespacedUniqueAndDescribed(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	for _, id := range builtin.All() {
		if !strings.HasPrefix(id, "gqaci.") {
			t.Errorf("%q is outside the gqaci. namespace", id)
		}
		if seen[id] {
			t.Errorf("%q is listed twice", id)
		}
		seen[id] = true
		if builtin.Describe(id) == "" {
			t.Errorf("%q has no description", id)
		}
	}
}

func TestAllIsSortedAndAReturnedCopy(t *testing.T) {
	t.Parallel()
	ids := builtin.All()
	if !sort.StringsAreSorted(ids) {
		t.Errorf("All is not sorted: %v", ids)
	}
	ids[0] = "mutated"
	if builtin.All()[0] == "mutated" {
		t.Error("All returns the internal slice")
	}
}

func TestDescribeUnknownIsEmpty(t *testing.T) {
	t.Parallel()
	if got := builtin.Describe("gqaci.nothing.here"); got != "" {
		t.Errorf("Describe of an unknown id = %q", got)
	}
}
