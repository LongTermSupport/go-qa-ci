package record_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/config"
	"github.com/LongTermSupport/go-qa-ci/internal/date"
	"github.com/LongTermSupport/go-qa-ci/internal/finding"
	"github.com/LongTermSupport/go-qa-ci/internal/record"
)

const (
	soundReason = "The generated parser module is regenerated from the grammar on every build."
	decider     = "alice"
)

// today is the fixed clock every case reviews against.
func today(t *testing.T) date.Date {
	t.Helper()
	d, err := date.Parse("2026-10-07")
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func intPtr(n int) *int { return &n }

func strPtr(s string) *string { return &s }

func testPolicy() config.RecordPolicy {
	p := config.Default().Record
	p.MaxTotal = 6
	p.MaxPerRule = 3
	p.MaxPolicy = 1
	p.Deciders = []string{decider}
	p.Trust.Models = map[string]config.Tier{"big": config.TierHigh, "mid": config.TierMedium, "small": config.TierLow}
	p.Trust.High = config.TierLimits{MaxReviewDays: intPtr(90)}
	p.Trust.Medium = config.TierLimits{MaxReviewDays: intPtr(30), MaxCount: intPtr(3)}
	p.Trust.Low = config.TierLimits{MaxReviewDays: intPtr(14), MaxCount: intPtr(1)}
	return p
}

// humanEntry is a valid exception decided by a human today, due for review in 60 days.
func humanEntry(t *testing.T, rule, path string) record.Exception {
	t.Helper()
	now := today(t)
	return record.Exception{
		Rule: rule, Path: path, Justification: soundReason,
		DecidedBy: strPtr(decider), AddedByModel: strPtr("human"),
		DecidedOn: now, ReviewBy: now.AddDays(60),
		Covers: record.CoversSuppressions,
	}
}

func review(t *testing.T, policy config.RecordPolicy, exceptions ...record.Exception) []finding.Finding {
	t.Helper()
	return record.Review(record.Input{
		Record: record.Record{Exceptions: exceptions}, Path: "qa/record.toml", Policy: policy, Today: today(t),
	})
}

func ids(findings []finding.Finding) []string {
	out := make([]string, 0, len(findings))
	for _, f := range findings {
		out = append(out, f.Rule)
	}
	return out
}

func assertOne(t *testing.T, findings []finding.Finding, rule, fragment string) {
	t.Helper()
	if len(findings) != 1 {
		t.Fatalf("want one finding, got %v", findings)
	}
	if findings[0].Rule != rule || !strings.Contains(findings[0].Message, fragment) {
		t.Errorf("got %s: %q, want %s containing %q", findings[0].Rule, findings[0].Message, rule, fragment)
	}
}

func numbered(t *testing.T, n int, rule string) []record.Exception {
	t.Helper()
	out := make([]record.Exception, 0, n)
	for i := range n {
		out = append(out, humanEntry(t, rule, fmt.Sprintf("internal/p%d/x.go", i)))
	}
	return out
}

func TestACleanRecordPassesAndAnEmptyOneToo(t *testing.T) {
	if got := review(t, testPolicy(), humanEntry(t, "errcheck", "a.go")); len(got) != 0 {
		t.Errorf("got %v", got)
	}
	if got := review(t, testPolicy()); len(got) != 0 {
		t.Errorf("got %v", got)
	}
}

func TestTotalBeyondTheBudgetFails(t *testing.T) {
	policy := testPolicy()
	policy.MaxPerRule = 6
	policy.MaxPolicy = 0
	found := false
	for _, f := range review(t, policy, numbered(t, 7, "errcheck")...) {
		found = found || (f.Rule == finding.RecordBudget && strings.Contains(f.Message, "7 exceptions exceed"))
	}
	if !found {
		t.Error("no budget finding for the total")
	}
}

func TestOneRuleBeyondItsCapFailsAndNamesTheRule(t *testing.T) {
	assertOne(t, review(t, testPolicy(), numbered(t, 4, "gosec G104")...), finding.RecordBudget, "gosec g104")
}

func TestPerRuleCountingIgnoresCaseAndSeparators(t *testing.T) {
	got := review(t, testPolicy(),
		humanEntry(t, "SA1019", "a.go"), humanEntry(t, "sa1019", "b.go"),
		humanEntry(t, "go-vet", "c.go"), humanEntry(t, "go_vet", "d.go"), humanEntry(t, "Go-Vet", "e.go"), humanEntry(t, " go-vet ", "f.go"),
	)
	if len(got) != 1 || got[0].Rule != finding.RecordBudget || !strings.Contains(got[0].Message, "go_vet") {
		t.Errorf("got %v", got)
	}
}

func TestPolicyLevelExceptionsPassWithinTheirOwnCapAndFailBeyond(t *testing.T) {
	a, b := humanEntry(t, "unused", "internal/?.go"), humanEntry(t, "unparam", "cmd/?.go")
	a.Policy, b.Policy = true, true
	if got := review(t, testPolicy(), a); len(got) != 0 {
		t.Errorf("one policy entry within cap: %v", got)
	}
	assertOne(t, review(t, testPolicy(), a, b), finding.RecordBudget, "policy-level")
}

func TestThePolicyMarkerOnASingleFileIsRejected(t *testing.T) {
	narrow := humanEntry(t, "unused", "a.go")
	narrow.Policy = true
	assertOne(t, review(t, testPolicy(), narrow), finding.RecordInvalid, "single file")
}

func TestAHighWaterAboveTheBudgetFails(t *testing.T) {
	got := record.Review(record.Input{
		Record: record.Record{Ratchet: &record.Ratchet{HighWater: 9}}, Path: "qa/record.toml", Policy: testPolicy(), Today: today(t),
	})
	assertOne(t, got, finding.RecordBudget, "high_water 9 exceeds max_total 6")
}

func TestLowerTiersHaveLowerCountCaps(t *testing.T) {
	smalls := numbered(t, 2, "errcheck")
	for i := range smalls {
		smalls[i].AddedByModel = strPtr("small")
		smalls[i].ReviewBy = today(t).AddDays(7)
	}
	assertOne(t, review(t, testPolicy(), smalls...), finding.RecordBudget, "low")
}

func TestMissingProvenanceFieldsAreEachReported(t *testing.T) {
	bare := humanEntry(t, "errcheck", "a.go")
	bare.DecidedBy, bare.AddedByModel, bare.DecidedOn, bare.ReviewBy = nil, nil, date.Date{}, date.Date{}
	got := review(t, testPolicy(), bare)
	if fmt.Sprint(ids(got)) != fmt.Sprint([]string{finding.RecordInvalid}) {
		t.Fatalf("got %v", got)
	}
	for _, field := range []string{"decided_by", "added_by_model", "decided_on", "review_by"} {
		if !strings.Contains(got[0].Message, field) {
			t.Errorf("%q lacks %s", got[0].Message, field)
		}
	}
}

func TestBlankProvenanceCountsAsMissing(t *testing.T) {
	blank := humanEntry(t, "errcheck", "a.go")
	blank.DecidedBy, blank.AddedByModel = strPtr("  "), strPtr("")
	assertOne(t, review(t, testPolicy(), blank), finding.RecordInvalid, "decided_by, added_by_model")
}

func TestAnExpiredReviewDateFailsUntilReDecided(t *testing.T) {
	old := humanEntry(t, "errcheck", "a.go")
	old.DecidedOn, old.ReviewBy = today(t).AddDays(-100), today(t).AddDays(-1)
	assertOne(t, review(t, testPolicy(), old), finding.RecordExpired, "re-decide")
	old.ReviewBy = today(t)
	if got := review(t, testPolicy(), old); len(got) != 0 {
		t.Errorf("due today still passes: %v", got)
	}
}

func TestAnEntryWithoutAReviewDateIsInvalidNotExpired(t *testing.T) {
	undated := humanEntry(t, "errcheck", "a.go")
	undated.ReviewBy = date.Date{}
	assertOne(t, review(t, testPolicy(), undated), finding.RecordInvalid, "review_by")
}

func TestAReviewDateBeyondTheHorizonIsRejected(t *testing.T) {
	far := humanEntry(t, "errcheck", "a.go")
	far.ReviewBy = today(t).AddDays(181)
	assertOne(t, review(t, testPolicy(), far), finding.RecordInvalid, "180 days")
	far.ReviewBy = today(t).AddDays(180)
	if got := review(t, testPolicy(), far); len(got) != 0 {
		t.Errorf("exactly at the horizon passes: %v", got)
	}
}

func TestADecisionDatedInTheFutureIsRejected(t *testing.T) {
	future := humanEntry(t, "errcheck", "a.go")
	future.DecidedOn, future.ReviewBy = today(t).AddDays(1), today(t).AddDays(10)
	assertOne(t, review(t, testPolicy(), future), finding.RecordInvalid, "future")
}

func TestAReviewDateBeforeTheDecisionIsRejected(t *testing.T) {
	odd := humanEntry(t, "errcheck", "a.go")
	odd.DecidedOn, odd.ReviewBy = today(t).AddDays(-10), today(t).AddDays(-20)
	found := false
	for _, f := range review(t, testPolicy(), odd) {
		found = found || (f.Rule == finding.RecordInvalid && strings.Contains(f.Message, "before"))
	}
	if !found {
		t.Error("no finding for review_by before decided_on")
	}
}

func TestFindingsCarryTheEntryLine(t *testing.T) {
	old := humanEntry(t, "errcheck", "a.go")
	old.ReviewBy, old.DecidedOn, old.Line = today(t).AddDays(-1), today(t).AddDays(-5), 7
	got := review(t, testPolicy(), old)
	if len(got) != 1 || got[0].Line != 7 || got[0].Path != "qa/record.toml" {
		t.Errorf("got %+v", got)
	}
}

func TestAnUnknownModelFailsTheGate(t *testing.T) {
	unknown := humanEntry(t, "errcheck", "a.go")
	unknown.AddedByModel = strPtr("mystery-1")
	assertOne(t, review(t, testPolicy(), unknown), finding.RecordInvalid, "mystery-1")
}

func TestAHumanEntryMustNameAConfiguredDecider(t *testing.T) {
	claimed := humanEntry(t, "errcheck", "a.go")
	claimed.DecidedBy = strPtr("mallory")
	assertOne(t, review(t, testPolicy(), claimed), finding.RecordInvalid, "deciders")
}

func TestWithNoDecidersConfiguredTheFindingSaysHowToConfigureThem(t *testing.T) {
	noDeciders := testPolicy()
	noDeciders.Deciders = nil
	assertOne(t, review(t, noDeciders, humanEntry(t, "errcheck", "a.go")), finding.RecordInvalid, "deciders = [")
	assertOne(t, review(t, noDeciders, humanEntry(t, "errcheck", "a.go")), finding.RecordInvalid, "[record]")
	if got := review(t, testPolicy(), func() record.Exception {
		e := humanEntry(t, "errcheck", "a.go")
		e.DecidedBy = strPtr("mallory")
		return e
	}()); len(got) != 1 || strings.Contains(got[0].Message, "deciders = [") {
		t.Errorf("the how-to-configure hint belongs only to a project with no deciders: %v", got)
	}
}

func TestLowerTiersGetShorterReviewHorizons(t *testing.T) {
	mid := humanEntry(t, "errcheck", "a.go")
	mid.AddedByModel = strPtr("mid-4")
	mid.ReviewBy = today(t).AddDays(30)
	if got := review(t, testPolicy(), mid); len(got) != 0 {
		t.Errorf("30 days passes for medium: %v", got)
	}
	mid.ReviewBy = today(t).AddDays(31)
	assertOne(t, review(t, testPolicy(), mid), finding.RecordInvalid, "30 days")
	small := humanEntry(t, "errcheck", "a.go")
	small.AddedByModel = strPtr("small-1")
	small.ReviewBy = today(t).AddDays(15)
	assertOne(t, review(t, testPolicy(), small), finding.RecordInvalid, "14 days")
}

func TestSecurityRulesFromLowTiersNeedAHumanCoSignature(t *testing.T) {
	policy := testPolicy()
	policy.Trust.Cosign = []config.Cosign{{Rules: []string{"gosec*", "govulncheck*"}, MinTier: config.TierHigh}}
	agent := humanEntry(t, "gosec G304", "a.go")
	agent.AddedByModel, agent.DecidedBy = strPtr("mid-4"), strPtr("mid-4")
	agent.ReviewBy = today(t).AddDays(10)
	assertOne(t, review(t, policy, agent), finding.RecordInvalid, "co-sign")
	agent.DecidedBy = strPtr(decider)
	if got := review(t, policy, agent); len(got) != 0 {
		t.Errorf("co-signed by a decider: %v", got)
	}
	high := agent
	high.AddedByModel, high.DecidedBy = strPtr("big-2"), strPtr("big-2")
	if got := review(t, policy, high); len(got) != 0 {
		t.Errorf("the high tier meets the threshold: %v", got)
	}
}

func TestRulesOutsideCosignCategoriesMayBeDecidedByAnAgent(t *testing.T) {
	agent := humanEntry(t, "errcheck", "a.go")
	agent.AddedByModel, agent.DecidedBy = strPtr("big-2"), strPtr("big-2")
	if got := review(t, testPolicy(), agent); len(got) != 0 {
		t.Errorf("got %v", got)
	}
}
