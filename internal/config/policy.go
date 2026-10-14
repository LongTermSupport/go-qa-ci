package config

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"unicode"

	"github.com/LongTermSupport/go-qa-ci/internal/glob"
)

// automationWords mark an identifier as an automation identity rather than a person.
var automationWords = map[string]bool{
	"bot": true, "bots": true, "agent": true, "agents": true, "ai": true, "llm": true,
	"model": true, "assistant": true, "automation": true, "automated": true, "ci": true,
	"actions": true, "dependabot": true, "renovate": true, "claude": true, "gpt": true,
	"chatgpt": true, "copilot": true, "codex": true, "gemini": true, "opus": true,
	"sonnet": true, "haiku": true, "llama": true,
}

// RankedTiers are the tiers that can hold entries, most trusted first.
func RankedTiers() []Tier {
	return []Tier{TierHuman, TierHigh, TierMedium, TierLow}
}

// Rank orders tiers by trust: a higher rank is more trusted. An unrecognised tier ranks lowest.
func (t Tier) Rank() int {
	switch t {
	case TierHuman:
		return 4
	case TierHigh:
		return 3
	case TierMedium:
		return 2
	case TierLow:
		return 1
	case TierUnknown:
		return 0
	default:
		return 0
	}
}

// TierOf is the tier for an added_by_model value: "human", else the longest configured model
// prefix that matches (case-insensitively), else TierUnknown.
func (p RecordPolicy) TierOf(model string) Tier {
	model = strings.ToLower(strings.TrimSpace(model))
	if model == "human" {
		return TierHuman
	}
	best, bestLen := TierUnknown, 0
	for _, prefix := range slices.Sorted(maps.Keys(p.Trust.Models)) {
		tier := p.Trust.Models[prefix]
		prefix = strings.ToLower(strings.TrimSpace(prefix))
		if prefix != "" && strings.HasPrefix(model, prefix) && len(prefix) > bestLen {
			best, bestLen = tier, len(prefix)
		}
	}
	return best
}

func (p RecordPolicy) limits(tier Tier) TierLimits {
	switch tier {
	case TierHuman:
		return p.Trust.Human
	case TierHigh:
		return p.Trust.High
	case TierMedium:
		return p.Trust.Medium
	case TierLow, TierUnknown:
		return p.Trust.Low
	default:
		return p.Trust.Low
	}
}

// ReviewDays is the effective review horizon in days for a tier: its own limit when set, never
// longer than max_review_days.
func (p RecordPolicy) ReviewDays(tier Tier) int {
	days := p.limits(tier).MaxReviewDays
	if days == nil {
		return p.MaxReviewDays
	}
	return min(*days, p.MaxReviewDays)
}

// CountCap is the number of entries a tier may hold, never more than max_total; false when the
// tier has no cap of its own (only max_total applies).
func (p RecordPolicy) CountCap(tier Tier) (int, bool) {
	count := p.limits(tier).MaxCount
	if count == nil {
		return 0, false
	}
	return min(*count, p.MaxTotal), true
}

// DecidersHint is the sentence that tells a project with no deciders how to configure them, and
// is empty once some are: a record cannot hold a human entry or a signed ratchet until then.
func (p RecordPolicy) DecidersHint() string {
	if len(p.Deciders) > 0 {
		return ""
	}
	return " (no deciders are configured: add deciders = [\"<name>\"] under [record] in " + DefaultPath + ")"
}

// IsDecider reports whether who is a configured decider.
func (p RecordPolicy) IsDecider(who string) bool {
	who = strings.TrimSpace(who)
	if who == "" {
		return false
	}
	for _, decider := range p.Deciders {
		if strings.TrimSpace(decider) == who {
			return true
		}
	}
	return false
}

// CosignThreshold is the highest min_tier among the cosign categories matching rule; false when
// no category matches. A malformed glob matches nothing (Problems reports it).
func (p RecordPolicy) CosignThreshold(rule string) (Tier, bool) {
	rule = strings.TrimSpace(rule)
	var best Tier
	found := false
	for _, category := range p.Trust.Cosign {
		for _, pattern := range category.Rules {
			if ok, err := glob.Match(pattern, rule); err != nil || !ok {
				continue
			}
			if !found || category.MinTier.Rank() > best.Rank() {
				best, found = category.MinTier, true
			}
			break
		}
	}
	return best, found
}

// LooksAutomated reports whether an identifier names an automation identity (a bot, CI job,
// agent or model).
func LooksAutomated(id string) bool {
	words := strings.FieldsFunc(strings.ToLower(id), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	for _, word := range words {
		if automationWords[word] {
			return true
		}
	}
	return false
}

// Problems checks the policy itself: human deciders, coherent caps, model mappings, cosign
// categories, and limits that get stricter for lower tiers.
func (p RecordPolicy) Problems() []string {
	var problems []string
	for _, decider := range p.Deciders {
		switch {
		case strings.TrimSpace(decider) == "":
			problems = append(problems, "record.deciders: empty identifier")
		case LooksAutomated(decider):
			problems = append(problems, fmt.Sprintf("record.deciders: `%s` looks like an automation identity; deciders must be people", decider))
		}
	}
	problems = append(problems, p.negativeProblems()...)
	if p.MaxPerRule > p.MaxTotal {
		problems = append(problems, fmt.Sprintf("record.max_per_rule (%d) exceeds record.max_total (%d)", p.MaxPerRule, p.MaxTotal))
	}
	if p.MaxPolicy > p.MaxPerRule {
		problems = append(problems, fmt.Sprintf("record.max_policy (%d) must not exceed record.max_per_rule (%d); policy-level exceptions have the stricter cap", p.MaxPolicy, p.MaxPerRule))
	}
	problems = append(problems, p.modelProblems()...)
	problems = append(problems, p.strictnessProblems()...)
	problems = append(problems, p.cosignProblems()...)
	return problems
}

func (p RecordPolicy) modelProblems() []string {
	var problems []string
	for _, prefix := range slices.Sorted(maps.Keys(p.Trust.Models)) {
		tier := p.Trust.Models[prefix]
		if strings.TrimSpace(prefix) == "" {
			problems = append(problems, "record.trust.models: empty model prefix")
		}
		switch tier {
		case TierHigh, TierMedium, TierLow:
		case TierHuman, TierUnknown:
			problems = append(problems, fmt.Sprintf("record.trust.models.%s maps to `%s`; models map to high, medium or low (`human` is reserved for people)", prefix, tier))
		default:
			problems = append(problems, fmt.Sprintf("record.trust.models.%s: unknown tier %q", prefix, tier))
		}
	}
	return problems
}

func (p RecordPolicy) strictnessProblems() []string {
	var problems []string
	ranked := RankedTiers()
	for i := 1; i < len(ranked); i++ {
		higher, lower := ranked[i-1], ranked[i]
		if p.ReviewDays(lower) > p.ReviewDays(higher) {
			problems = append(problems, fmt.Sprintf("record.trust.%s.max_review_days (%d) is longer than trust.%s (%d); lower tiers must be stricter",
				lower, p.ReviewDays(lower), higher, p.ReviewDays(higher)))
		}
		if p.capOrTotal(lower) > p.capOrTotal(higher) {
			problems = append(problems, fmt.Sprintf("record.trust.%s.max_count (%d) is larger than trust.%s (%d); lower tiers must be stricter",
				lower, p.capOrTotal(lower), higher, p.capOrTotal(higher)))
		}
	}
	return problems
}

func (p RecordPolicy) capOrTotal(tier Tier) int {
	if count, ok := p.CountCap(tier); ok {
		return count
	}
	return p.MaxTotal
}

func (p RecordPolicy) cosignProblems() []string {
	var problems []string
	for _, category := range p.Trust.Cosign {
		for _, pattern := range category.Rules {
			if err := glob.Valid(pattern); err != nil {
				problems = append(problems, fmt.Sprintf("record.trust.cosign: invalid rule glob `%s`", pattern))
			}
		}
		switch {
		case !category.MinTier.Valid():
			problems = append(problems, fmt.Sprintf("record.trust.cosign: unknown min_tier %q", category.MinTier))
		case category.MinTier == TierUnknown:
			problems = append(problems, "record.trust.cosign: min_tier cannot be `unknown`")
		}
	}
	return problems
}

// negativeProblems rejects a negative cap or horizon, which would make every non-empty record
// fail with a budget message instead of a configuration error.
func (p RecordPolicy) negativeProblems() []string {
	var problems []string
	for _, c := range []struct {
		name  string
		value int
	}{{"max_total", p.MaxTotal}, {"max_per_rule", p.MaxPerRule}, {"max_policy", p.MaxPolicy}, {"max_review_days", p.MaxReviewDays}} {
		if c.value < 0 {
			problems = append(problems, fmt.Sprintf("record.%s (%d) must not be negative", c.name, c.value))
		}
	}
	for _, t := range []struct {
		name   string
		limits TierLimits
	}{{"human", p.Trust.Human}, {"high", p.Trust.High}, {"medium", p.Trust.Medium}, {"low", p.Trust.Low}} {
		if d := t.limits.MaxReviewDays; d != nil && *d < 0 {
			problems = append(problems, fmt.Sprintf("record.trust.%s.max_review_days (%d) must not be negative", t.name, *d))
		}
		if c := t.limits.MaxCount; c != nil && *c < 0 {
			problems = append(problems, fmt.Sprintf("record.trust.%s.max_count (%d) must not be negative", t.name, *c))
		}
	}
	return problems
}
