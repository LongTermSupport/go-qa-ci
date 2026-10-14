package record_test

import (
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/date"
	"github.com/LongTermSupport/go-qa-ci/internal/finding"
	"github.com/LongTermSupport/go-qa-ci/internal/record"
)

func signed(t *testing.T, highWater int, decidedOn date.Date, entries ...string) record.Record {
	t.Helper()
	rec := record.Record{Ratchet: &record.Ratchet{
		HighWater: highWater, DecidedBy: decider, DecidedOn: decidedOn, Reason: soundReason,
	}}
	for _, path := range entries {
		rec.Exceptions = append(rec.Exceptions, humanEntry(t, "errcheck", path))
	}
	return rec
}

func ratchetOf(t *testing.T, rec record.Record, previous *record.Record) record.RatchetVerdict {
	t.Helper()
	return record.ReviewRatchet(record.Input{Record: rec, Path: "qa/record.toml", Policy: testPolicy(), Today: today(t)}, previous)
}

func TestCeilingIsTheRatchetOrTheSize(t *testing.T) {
	if got := record.Ceiling(signed(t, 4, today(t), "a.go")); got != 4 {
		t.Errorf("signed: %d", got)
	}
	if got := record.Ceiling(record.Record{Exceptions: signed(t, 0, today(t), "a.go", "b.go").Exceptions}); got != 2 {
		t.Errorf("unsigned: %d", got)
	}
}

func TestAnEmptyRecordNeedsNoRatchet(t *testing.T) {
	if v := ratchetOf(t, record.Record{}, nil); len(v.Findings) != 0 || v.LowerTo != nil {
		t.Errorf("got %+v", v)
	}
}

func TestEntriesWithoutARatchetFail(t *testing.T) {
	rec := signed(t, 0, today(t), "a.go")
	rec.Ratchet = nil
	assertOne(t, ratchetOf(t, rec, nil).Findings, finding.RecordRatchet, "no [ratchet] table")
}

func TestTheRatchetMustBeSignedByADeciderWithAReason(t *testing.T) {
	rec := signed(t, 1, today(t), "a.go")
	rec.Ratchet.DecidedBy = "ci-bot"
	rec.Ratchet.Reason = "todo"
	assertOne(t, ratchetOf(t, rec, nil).Findings, finding.RecordRatchet, "ci-bot")
	if f := ratchetOf(t, rec, nil).Findings[0]; f.Message == "" || f.Path != "qa/record.toml" {
		t.Errorf("got %+v", f)
	}
}

func TestAFutureSignatureFails(t *testing.T) {
	rec := signed(t, 1, today(t).AddDays(1), "a.go")
	assertOne(t, ratchetOf(t, rec, nil).Findings, finding.RecordRatchet, "in the future")
}

func TestGrowingPastTheCeilingFails(t *testing.T) {
	rec := signed(t, 1, today(t), "a.go", "b.go")
	assertOne(t, ratchetOf(t, rec, nil).Findings, finding.RecordRatchet, "grew to 2")
}

func TestShrinkingAsksForALowerCeiling(t *testing.T) {
	v := ratchetOf(t, signed(t, 3, today(t), "a.go"), nil)
	if len(v.Findings) != 0 || v.LowerTo == nil || *v.LowerTo != 1 {
		t.Errorf("got %+v", v)
	}
}

func TestARaiseNeedsAFreshSignatureNewerThanTheLast(t *testing.T) {
	before := signed(t, 1, today(t), "a.go")
	after := signed(t, 2, today(t), "a.go", "b.go")
	assertOne(t, ratchetOf(t, after, &before).Findings, finding.RecordRatchet, "raised from 1 to 2")

	oldBefore := signed(t, 1, today(t).AddDays(-60), "a.go")
	stale := signed(t, 2, today(t).AddDays(-30), "a.go", "b.go")
	assertOne(t, ratchetOf(t, stale, &oldBefore).Findings, finding.RecordRatchet, "14 days")

	fresh := signed(t, 2, today(t), "a.go", "b.go")
	if got := ratchetOf(t, fresh, &oldBefore).Findings; len(got) != 0 {
		t.Errorf("fresh signature rejected: %v", got)
	}
}

func TestASecondRaiseOnTheSameDayNeedsANewDecision(t *testing.T) {
	before := signed(t, 1, today(t), "a.go")
	same := signed(t, 2, today(t), "a.go", "b.go")
	assertOne(t, ratchetOf(t, same, &before).Findings, finding.RecordRatchet, "raised from 1 to 2")

	reworded := signed(t, 2, today(t), "a.go", "b.go")
	reworded.Ratchet.Reason = "A second entry is needed for the generated client, which the first decision did not cover."
	if got := ratchetOf(t, reworded, &before).Findings; len(got) != 0 {
		t.Errorf("a re-signed raise on the same day was rejected: %v", got)
	}
}

func TestAdoptingTheRatchetAtTheExistingCountIsNotARaise(t *testing.T) {
	legacy := signed(t, 0, today(t), "a.go")
	legacy.Ratchet = nil
	adopted := signed(t, 1, today(t).AddDays(-100), "a.go")
	if got := ratchetOf(t, adopted, &legacy).Findings; len(got) != 0 {
		t.Errorf("got %v", got)
	}
}
