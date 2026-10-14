package record

import (
	"fmt"
	"maps"
	"slices"

	"github.com/LongTermSupport/go-qa-ci/internal/config"
)

// Budget is the record's size against its anti-bloat caps (TOOLING-SPEC 6.3).
type Budget struct {
	// Total is the number of exceptions; MaxTotal is its cap.
	Total    int `json:"total"`
	MaxTotal int `json:"max_total"`
	// BusiestRule is the rule with most exceptions and BusiestCount how many it has; MaxPerRule is the cap.
	BusiestRule  string `json:"busiest_rule"`
	BusiestCount int    `json:"busiest_count"`
	MaxPerRule   int    `json:"max_per_rule"`
	// Policy counts the broad (glob) exceptions; MaxPolicy is its cap.
	Policy    int `json:"policy"`
	MaxPolicy int `json:"max_policy"`
	// HighWater is the ratchet ceiling; -1 when the record has no ratchet.
	HighWater int `json:"high_water"`
	// MaxReviewDays is the longest review horizon for any tier.
	MaxReviewDays int `json:"max_review_days"`
}

// BudgetOf measures rec against policy.
func BudgetOf(rec Record, policy config.RecordPolicy) Budget {
	perRule := map[string]int{}
	for _, e := range rec.Exceptions {
		perRule[normaliseRule(e.Rule)]++
	}
	busiest, count := "", 0
	for _, rule := range slices.Sorted(maps.Keys(perRule)) {
		if perRule[rule] > count {
			busiest, count = rule, perRule[rule]
		}
	}
	highWater := -1
	if rec.Ratchet != nil {
		highWater = rec.Ratchet.HighWater
	}
	return Budget{
		Total: len(rec.Exceptions), MaxTotal: policy.MaxTotal,
		BusiestRule: busiest, BusiestCount: count, MaxPerRule: policy.MaxPerRule,
		Policy: countWhere(rec.Exceptions, func(e Exception) bool { return e.Policy }), MaxPolicy: policy.MaxPolicy,
		HighWater: highWater, MaxReviewDays: policy.MaxReviewDays,
	}
}

// Lines states the budget one fact per line, for humans and the agent summary.
func (b Budget) Lines() []string {
	perRule := "none yet"
	if b.BusiestCount > 0 {
		perRule = fmt.Sprintf("busiest `%s` with %d", b.BusiestRule, b.BusiestCount)
	}
	ratchet := "none"
	if b.HighWater >= 0 {
		ratchet = fmt.Sprintf("high_water %d", b.HighWater)
	}
	return []string{
		fmt.Sprintf("exceptions: %d of %d", b.Total, b.MaxTotal),
		fmt.Sprintf("per rule: at most %d (%s)", b.MaxPerRule, perRule),
		fmt.Sprintf("policy-level (glob): %d of %d", b.Policy, b.MaxPolicy),
		"ratchet: " + ratchet,
		fmt.Sprintf("review horizon: %d days", b.MaxReviewDays),
	}
}
