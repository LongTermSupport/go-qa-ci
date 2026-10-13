package record

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/LongTermSupport/go-qa-ci/internal/config"
	"github.com/LongTermSupport/go-qa-ci/internal/date"
	"github.com/LongTermSupport/go-qa-ci/internal/finding"
	"github.com/LongTermSupport/go-qa-ci/internal/glob"
)

// Input is what a policy review looks at.
type Input struct {
	// Record is the project record as it is now.
	Record Record
	// Path is the record's path as the project configures it; findings show it.
	Path string
	// Policy is the [record] table of the configuration.
	Policy config.RecordPolicy
	// Today is the date reviews and deadlines are measured against; the caller supplies the clock.
	Today date.Date
}

// Review applies the anti-bloat controls (TOOLING-SPEC 6.4): provenance, trust tiers, review
// horizons and co-signing per entry (gqaci.record.invalid), expiry (gqaci.record.expired) and the
// budgets (gqaci.record.budget). The ratchet is a separate control.
func Review(in Input) []finding.Finding {
	var findings []finding.Finding
	for i, e := range in.Record.Exceptions {
		findings = append(findings, reviewEntry(in, i, e)...)
	}
	return append(findings, reviewBudget(in)...)
}

func (in Input) finding(rule string, line int, message string) finding.Finding {
	return finding.Finding{Rule: rule, Path: in.Path, Line: line, Message: message}
}

// present returns the trimmed value, or "" when it is absent or blank.
func present(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

// normaliseRule makes spellings of one identifier count as one: trimmed, lower-case, with hyphens
// read as underscores.
func normaliseRule(rule string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(rule)), "-", "_")
}

func reviewEntry(in Input, index int, e Exception) []finding.Finding {
	label := fmt.Sprintf("exception #%d (rule `%s`, path `%s`)", index+1, e.Rule, e.Path)
	var findings []finding.Finding
	if problems := entryProblems(in, e); len(problems) > 0 {
		findings = append(findings, in.finding(finding.RecordInvalid, e.Line, label+" "+strings.Join(problems, "; ")))
	}
	if !e.ReviewBy.IsZero() && e.ReviewBy.Before(in.Today) {
		findings = append(findings, in.finding(finding.RecordExpired, e.Line, fmt.Sprintf(
			"%s was due for review on %s; re-decide it (fresh decided_on and review_by, justification re-checked) or remove it and fix the code",
			label, e.ReviewBy)))
	}
	return findings
}

func entryProblems(in Input, e Exception) []string {
	decidedBy, model := present(e.DecidedBy), present(e.AddedByModel)
	var problems []string
	if missing := missingFields(e, decidedBy, model); len(missing) > 0 {
		problems = append(problems, "is missing "+strings.Join(missing, ", "))
	}
	tier := config.TierLow
	if model != "" {
		tier = in.Policy.TierOf(model)
		problems = append(problems, trustProblems(in.Policy, e.Rule, model, decidedBy, tier)...)
		if tier == config.TierUnknown {
			tier = config.TierLow
		}
	}
	problems = append(problems, dateProblems(in, e, tier)...)
	if e.Policy && !glob.IsGlob(e.Path) {
		problems = append(problems, "is marked policy = true but names a single file; drop the marker")
	}
	return problems
}

func missingFields(e Exception, decidedBy, model string) []string {
	var missing []string
	for _, field := range []struct {
		name   string
		absent bool
	}{
		{"decided_by", decidedBy == ""},
		{"added_by_model", model == ""},
		{"decided_on", e.DecidedOn.IsZero()},
		{"review_by", e.ReviewBy.IsZero()},
	} {
		if field.absent {
			missing = append(missing, field.name)
		}
	}
	return missing
}

// trustProblems judges the author's tier: an unmapped model is rejected, a human mark needs a
// configured decider, and a rule in a co-sign category written below its threshold needs one too.
func trustProblems(policy config.RecordPolicy, rule, model, decidedBy string, tier config.Tier) []string {
	coSigned := decidedBy != "" && policy.IsDecider(decidedBy)
	switch tier {
	case config.TierUnknown:
		return []string{fmt.Sprintf("added_by_model `%s` maps to no trust tier; list its prefix under [record.trust] models, or write \"human\"", model)}
	case config.TierHuman:
		if decidedBy != "" && !coSigned {
			return []string{fmt.Sprintf("is marked human but decided_by `%s` is not one of the configured deciders", decidedBy)}
		}
	case config.TierHigh, config.TierMedium, config.TierLow:
		if threshold, ok := policy.CosignThreshold(rule); ok && tier.Rank() < threshold.Rank() && !coSigned {
			return []string{fmt.Sprintf("needs a human co-sign: written by a %s-tier model below the `%s` threshold for this rule, so decided_by must be one of the configured deciders", tier, threshold)}
		}
	}
	return nil
}

func dateProblems(in Input, e Exception, tier config.Tier) []string {
	var problems []string
	if !e.DecidedOn.IsZero() && e.DecidedOn.After(in.Today) {
		problems = append(problems, fmt.Sprintf("decided_on %s is in the future", e.DecidedOn))
	}
	if e.DecidedOn.IsZero() || e.ReviewBy.IsZero() {
		return problems
	}
	horizon := in.Policy.ReviewDays(tier)
	switch {
	case e.ReviewBy.Before(e.DecidedOn):
		problems = append(problems, fmt.Sprintf("review_by %s is before decided_on %s", e.ReviewBy, e.DecidedOn))
	case e.DecidedOn.DaysUntil(e.ReviewBy) > horizon:
		problems = append(problems, fmt.Sprintf("review_by %s is more than %d days after decided_on %s (the limit for the %s tier)",
			e.ReviewBy, horizon, e.DecidedOn, tier))
	}
	return problems
}

func reviewBudget(in Input) []finding.Finding {
	policy, exceptions := in.Policy, in.Record.Exceptions
	var findings []finding.Finding
	add := func(format string, args ...any) {
		findings = append(findings, in.finding(finding.RecordBudget, 0, fmt.Sprintf(format, args...)))
	}
	if len(exceptions) > policy.MaxTotal {
		add("%d exceptions exceed max_total %d; fix the code behind some entries and remove them", len(exceptions), policy.MaxTotal)
	}
	perRule := map[string]int{}
	for _, e := range exceptions {
		perRule[normaliseRule(e.Rule)]++
	}
	for _, rule := range slices.Sorted(maps.Keys(perRule)) {
		if perRule[rule] > policy.MaxPerRule {
			add("rule `%s` has %d exceptions, over max_per_rule %d; fix the pattern at its source instead of excepting each site",
				rule, perRule[rule], policy.MaxPerRule)
		}
	}
	if broad := countWhere(exceptions, func(e Exception) bool { return e.Policy }); broad > policy.MaxPolicy {
		add("%d policy-level (glob) exceptions exceed max_policy %d; narrow them to single files", broad, policy.MaxPolicy)
	}
	for _, tier := range config.RankedTiers() {
		limit, capped := policy.CountCap(tier)
		count := countWhere(exceptions, func(e Exception) bool {
			model := present(e.AddedByModel)
			return model != "" && policy.TierOf(model) == tier
		})
		if capped && count > limit {
			add("%d exceptions written by the %s tier exceed trust.%s.max_count %d", count, tier, tier, limit)
		}
	}
	if r := in.Record.Ratchet; r != nil && r.HighWater > policy.MaxTotal {
		add("[ratchet] high_water %d exceeds max_total %d", r.HighWater, policy.MaxTotal)
	}
	return findings
}

func countWhere(exceptions []Exception, keep func(Exception) bool) int {
	n := 0
	for _, e := range exceptions {
		if keep(e) {
			n++
		}
	}
	return n
}
