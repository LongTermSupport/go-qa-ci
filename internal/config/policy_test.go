package config_test

import (
	"strings"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/config"
)

func ptr(n int) *int { return &n }

// testPolicy is the policy the record tests share: three model classes, one decider.
func testPolicy() config.RecordPolicy {
	p := config.Default().Record
	p.MaxTotal = 6
	p.MaxPerRule = 3
	p.MaxPolicy = 1
	p.Deciders = []string{"alice"}
	p.Trust.Models = map[string]config.Tier{"big": config.TierHigh, "mid": config.TierMedium, "small": config.TierLow}
	p.Trust.High = config.TierLimits{MaxReviewDays: ptr(90)}
	p.Trust.Medium = config.TierLimits{MaxReviewDays: ptr(30), MaxCount: ptr(3)}
	p.Trust.Low = config.TierLimits{MaxReviewDays: ptr(14), MaxCount: ptr(1)}
	return p
}

func TestModelsMapToTiersByLongestPrefixAndHumanIsReserved(t *testing.T) {
	p := testPolicy()
	p.Trust.Models["big-lite"] = config.TierLow
	cases := map[string]config.Tier{
		"human": config.TierHuman, "Human": config.TierHuman,
		"big-5-5": config.TierHigh, "BIG-5": config.TierHigh, "big-lite-2": config.TierLow,
		"mid-4": config.TierMedium, "other-model": config.TierUnknown, "": config.TierUnknown,
	}
	for model, want := range cases {
		if got := p.TierOf(model); got != want {
			t.Errorf("TierOf(%q) = %s, want %s", model, got, want)
		}
	}
}

func TestEmptyPrefixMatchesNothing(t *testing.T) {
	p := testPolicy()
	p.Trust.Models[" "] = config.TierHigh
	if got := p.TierOf("anything"); got != config.TierUnknown {
		t.Errorf("TierOf = %s", got)
	}
}

func TestTierRankOrdersByTrust(t *testing.T) {
	order := []config.Tier{config.TierUnknown, config.TierLow, config.TierMedium, config.TierHigh, config.TierHuman}
	for i := 1; i < len(order); i++ {
		if order[i-1].Rank() >= order[i].Rank() {
			t.Errorf("%s should rank below %s", order[i-1], order[i])
		}
	}
}

func TestReviewDaysAndCountCapFallBackToTheRecordWideCaps(t *testing.T) {
	p := testPolicy()
	if got := p.ReviewDays(config.TierHuman); got != 180 {
		t.Errorf("human horizon = %d, want the record-wide 180", got)
	}
	if got := p.ReviewDays(config.TierMedium); got != 30 {
		t.Errorf("medium horizon = %d", got)
	}
	if got := p.ReviewDays(config.TierUnknown); got != 14 {
		t.Errorf("unknown reads as low: %d", got)
	}
	p.Trust.High.MaxReviewDays = ptr(400)
	if got := p.ReviewDays(config.TierHigh); got != 180 {
		t.Errorf("a tier horizon never exceeds max_review_days: %d", got)
	}
	if _, ok := p.CountCap(config.TierHigh); ok {
		t.Error("high has no cap of its own")
	}
	p.Trust.Medium.MaxCount = ptr(50)
	if got, ok := p.CountCap(config.TierMedium); !ok || got != 6 {
		t.Errorf("a tier cap never exceeds max_total: %d %v", got, ok)
	}
}

func TestIsDeciderNeedsAnExactNonBlankName(t *testing.T) {
	p := testPolicy()
	if !p.IsDecider("alice") || !p.IsDecider(" alice ") || p.IsDecider("mallory") || p.IsDecider(" ") {
		t.Error("IsDecider wrong")
	}
}

func TestCosignThresholdIsTheHighestMatchingCategory(t *testing.T) {
	p := testPolicy()
	p.Trust.Cosign = []config.Cosign{
		{Rules: []string{"gosec*"}, MinTier: config.TierMedium},
		{Rules: []string{"gosec G1*", "SA5*"}, MinTier: config.TierHigh},
	}
	if got, ok := p.CosignThreshold("gosec G104"); !ok || got != config.TierHigh {
		t.Errorf("got %s %v", got, ok)
	}
	if got, ok := p.CosignThreshold("gosecurity"); !ok || got != config.TierMedium {
		t.Errorf("got %s %v", got, ok)
	}
	if _, ok := p.CosignThreshold("errcheck"); ok {
		t.Error("errcheck is in no category")
	}
}

func TestLooksAutomatedRecognisesAutomationIdentities(t *testing.T) {
	for _, id := range []string{"ci-bot", "dependabot[bot]", "github-actions", "release_agent", "Claude", "gpt", "renovate"} {
		if !config.LooksAutomated(id) {
			t.Errorf("%s should look automated", id)
		}
	}
	// "maintainer" and "Decider" hold "ai" and "ci" inside a longer word, which is not a match.
	for _, id := range []string{"alice", "bob", "maintainer", "Decider"} {
		if config.LooksAutomated(id) {
			t.Errorf("%s should look human", id)
		}
	}
}

func TestPolicyValidationRejectsUnsafeSettings(t *testing.T) {
	if problems := testPolicy().Problems(); len(problems) != 0 {
		t.Fatalf("the shared policy is invalid: %v", problems)
	}
	cases := map[string]func(*config.RecordPolicy){
		"automation": func(p *config.RecordPolicy) { p.Deciders = append(p.Deciders, "deploy-bot") },
		"empty":      func(p *config.RecordPolicy) { p.Deciders = append(p.Deciders, " ") },
		"human":      func(p *config.RecordPolicy) { p.Trust.Models["x"] = config.TierHuman },
		"unknown":    func(p *config.RecordPolicy) { p.Trust.Models["x"] = config.TierUnknown },
		"prefix":     func(p *config.RecordPolicy) { p.Trust.Models[" "] = config.TierLow },
		"stricter": func(p *config.RecordPolicy) {
			p.Trust.Low.MaxReviewDays = ptr(60)
		},
		"max_count": func(p *config.RecordPolicy) { p.Trust.Low.MaxCount = ptr(5) },
		"glob": func(p *config.RecordPolicy) {
			p.Trust.Cosign = append(p.Trust.Cosign, config.Cosign{Rules: []string{"["}, MinTier: config.TierHigh})
		},
		"min_tier": func(p *config.RecordPolicy) {
			p.Trust.Cosign = append(p.Trust.Cosign, config.Cosign{Rules: []string{"x*"}, MinTier: config.TierUnknown})
		},
	}
	for fragment, mutate := range cases {
		p := testPolicy()
		mutate(&p)
		joined := strings.Join(p.Problems(), "; ")
		if !strings.Contains(joined, fragment) {
			t.Errorf("%q lacks %q", joined, fragment)
		}
	}
}
