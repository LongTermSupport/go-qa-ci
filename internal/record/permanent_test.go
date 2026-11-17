package record_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/config"
	"github.com/LongTermSupport/go-qa-ci/internal/date"
	"github.com/LongTermSupport/go-qa-ci/internal/finding"
	"github.com/LongTermSupport/go-qa-ci/internal/record"
)

// permanentEntry is a valid permanent errcheck exception at path: decided by a decider, written by
// a medium-tier model, with no review date.
func permanentEntry(t *testing.T, path string) record.Exception {
	t.Helper()
	e := humanEntry(t, "errcheck", path)
	e.AddedByModel = strPtr("mid-model")
	e.ReviewBy = date.Date{}
	e.Permanent = true
	return e
}

func TestLoadReadsPermanent(t *testing.T) {
	path := write(t, `[[exception]]
rule = "errcheck"
path = "src/a.go"
justification = "`+good+`"
decided_by = "owner"
added_by_model = "human"
decided_on = 2026-10-01
permanent = true
`)
	loaded, err := record.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Exceptions) != 1 || !loaded.Exceptions[0].Permanent || !loaded.Exceptions[0].ReviewBy.IsZero() {
		t.Errorf("got %+v", loaded.Exceptions)
	}
}

func TestAPermanentExceptionIsValidAndNeverExpires(t *testing.T) {
	e := permanentEntry(t, "a.go")
	e.DecidedOn = today(t).AddDays(-3650)
	if got := review(t, testPolicy(), e); len(got) != 0 {
		t.Errorf("got %v", got)
	}
}

func TestPermanentWithAReviewDateIsInvalid(t *testing.T) {
	e := permanentEntry(t, "a.go")
	e.ReviewBy = today(t).AddDays(10)
	assertOne(t, review(t, testPolicy(), e), finding.RecordInvalid, "permanent = true and a review_by")
}

func TestAnExpiredReviewDateOnAPermanentEntryIsInvalidNotExpired(t *testing.T) {
	e := permanentEntry(t, "a.go")
	e.DecidedOn = today(t).AddDays(-20)
	e.ReviewBy = today(t).AddDays(-1)
	assertOne(t, review(t, testPolicy(), e), finding.RecordInvalid, "permanent = true and a review_by")
}

func TestNeitherPermanentNorAReviewDateIsInvalid(t *testing.T) {
	e := humanEntry(t, "errcheck", "a.go")
	e.ReviewBy = date.Date{}
	assertOne(t, review(t, testPolicy(), e), finding.RecordInvalid, "review_by (or permanent = true)")
}

func TestPermanentStillNeedsItsProvenance(t *testing.T) {
	e := permanentEntry(t, "a.go")
	e.DecidedOn = date.Date{}
	e.AddedByModel = nil
	got := review(t, testPolicy(), e)
	assertOne(t, got, finding.RecordInvalid, "is missing added_by_model, decided_on")
	if strings.Contains(got[0].Message, "review_by") {
		t.Errorf("a permanent entry was asked for a review_by: %q", got[0].Message)
	}
}

func TestPermanentMustBeDecidedByADecider(t *testing.T) {
	e := permanentEntry(t, "a.go")
	e.DecidedBy = strPtr("bob")
	assertOne(t, review(t, testPolicy(), e), finding.RecordInvalid, "only a configured decider can make an exception permanent")
}

func TestPermanentCountsTowardTheBudgetAndTheRatchet(t *testing.T) {
	policy := testPolicy()
	policy.MaxPerRule = 6
	policy.MaxPolicy = 0
	entries := numbered(t, 6, "errcheck")
	entries = append(entries, permanentEntry(t, "z.go"))
	found := false
	for _, f := range review(t, policy, entries...) {
		found = found || (f.Rule == finding.RecordBudget && strings.Contains(f.Message, "7 exceptions exceed"))
	}
	if !found {
		t.Error("the permanent entry was not counted toward max_total")
	}
	rec := record.Record{
		Ratchet:    &record.Ratchet{HighWater: 1, DecidedBy: decider, DecidedOn: today(t), Reason: soundReason},
		Exceptions: []record.Exception{humanEntry(t, "errcheck", "a.go"), permanentEntry(t, "b.go")},
	}
	budget := record.BudgetOf(rec, config.Default().Record)
	if budget.Total != 2 || budget.Permanent != 1 {
		t.Errorf("budget %+v", budget)
	}
}

func TestBudgetLinesShowThePermanentCount(t *testing.T) {
	rec := record.Record{Exceptions: []record.Exception{permanentEntry(t, "a.go")}}
	got := record.BudgetOf(rec, config.Default().Record).Lines()
	want := []string{
		"exceptions: 1 of 10",
		"per rule: at most 3 (busiest `errcheck` with 1)",
		"policy-level (glob): 0 of 0",
		"permanent (no review date): 1",
		"ratchet: none",
		"review horizon: 180 days",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestListShowsPermanentInTextAndJSON(t *testing.T) {
	rec := record.Record{Exceptions: []record.Exception{permanentEntry(t, "a.go"), humanEntry(t, "govet", "b.go")}}
	text := rec.List()
	if !strings.Contains(text, "#1 errcheck  a.go  covers suppressions  permanent") ||
		!strings.Contains(text, "review_by never (permanent)") {
		t.Errorf("list:\n%s", text)
	}
	if strings.Contains(text, "#2 govet  b.go  covers suppressions  permanent") {
		t.Errorf("a dated entry is listed as permanent:\n%s", text)
	}
	explained, err := rec.Explain("1")
	if err != nil || !strings.Contains(explained, "permanent: true") {
		t.Errorf("explain %q, %v", explained, err)
	}
	data, err := rec.ListJSON()
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Exceptions []map[string]any `json:"exceptions"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Exceptions[0]["permanent"] != true || decoded.Exceptions[1]["permanent"] != false {
		t.Errorf("got %s", data)
	}
	if _, has := decoded.Exceptions[0]["review_by"]; has {
		t.Errorf("a permanent entry lists a review_by: %s", data)
	}
}
