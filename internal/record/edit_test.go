package record_test

import (
	"strings"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/record"
)

const editable = `# header comment

[ratchet]
high_water = 3  # ceiling
decided_by = "alice"

# first entry
[[exception]]
rule = "a"
path = "a.go"

# second entry
[[exception]]
rule = "b"
path = "b.go"

[[exception]]
rule = "c"
path = "c.go"
`

func TestSetHighWaterRewritesOnlyTheRatchetValue(t *testing.T) {
	out, ok := record.SetHighWater(editable, 1)
	if !ok || !strings.Contains(out, "high_water = 1  # ceiling") || !strings.Contains(out, "# header comment") {
		t.Errorf("got %v\n%s", ok, out)
	}
	if strings.Count(out, "high_water") != 1 {
		t.Errorf("other keys touched:\n%s", out)
	}
}

func TestSetHighWaterIgnoresOtherTables(t *testing.T) {
	text := "[other]\nhigh_water = 9\n"
	if _, ok := record.SetHighWater(text, 1); ok {
		t.Error("rewrote a key outside [ratchet]")
	}
}

func TestRemoveExceptionsDropsTheEntryAndItsComment(t *testing.T) {
	out := record.RemoveExceptions(editable, []int{1})
	if strings.Contains(out, `rule = "b"`) || strings.Contains(out, "second entry") {
		t.Errorf("entry survived:\n%s", out)
	}
	for _, keep := range []string{"first entry", `rule = "a"`, `rule = "c"`, "[ratchet]"} {
		if !strings.Contains(out, keep) {
			t.Errorf("lost %q:\n%s", keep, out)
		}
	}
	if _, err := record.Parse([]byte(out)); err != nil {
		t.Errorf("result does not parse: %v", err)
	}
}

func TestRemoveExceptionsIgnoresOutOfRangeIndexes(t *testing.T) {
	if out := record.RemoveExceptions(editable, []int{7}); out != editable {
		t.Errorf("changed:\n%s", out)
	}
}
