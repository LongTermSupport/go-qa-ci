package record_test

import (
	"reflect"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/config"
	"github.com/LongTermSupport/go-qa-ci/internal/record"
)

func TestBudgetCountsAgainstTheCaps(t *testing.T) {
	rec := sample()
	rec.Exceptions = append(rec.Exceptions, entry("errcheck", "src/c.go", good))
	got := record.BudgetOf(rec, config.Default().Record)
	want := record.Budget{
		Total: 3, MaxTotal: 10, BusiestRule: "errcheck", BusiestCount: 2, MaxPerRule: 3,
		Policy: 0, MaxPolicy: 0, HighWater: 2, MaxReviewDays: 180,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestBudgetLinesStateEveryCapForAnEmptyRecord(t *testing.T) {
	got := record.BudgetOf(record.Record{}, config.Default().Record).Lines()
	want := []string{
		"exceptions: 0 of 10",
		"per rule: at most 3 (none yet)",
		"policy-level (glob): 0 of 0",
		"ratchet: none",
		"review horizon: 180 days",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}
