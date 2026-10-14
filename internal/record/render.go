package record

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// ListedRatchet is the ratchet as the listing shows it.
type ListedRatchet struct {
	HighWater int    `json:"high_water"`
	DecidedBy string `json:"decided_by"`
	DecidedOn string `json:"decided_on,omitempty"`
	Reason    string `json:"reason"`
}

// ListedException is one exception as the listing shows it.
type ListedException struct {
	Number        int    `json:"number"`
	Rule          string `json:"rule"`
	Path          string `json:"path"`
	Covers        Covers `json:"covers"`
	Policy        bool   `json:"policy"`
	DecidedBy     string `json:"decided_by,omitempty"`
	AddedByModel  string `json:"added_by_model,omitempty"`
	DecidedOn     string `json:"decided_on,omitempty"`
	ReviewBy      string `json:"review_by,omitempty"`
	Justification string `json:"justification"`
}

type listing struct {
	Ratchet    *ListedRatchet    `json:"ratchet,omitempty"`
	Exceptions []ListedException `json:"exceptions"`
}

func (r Record) listing() listing {
	out := listing{Exceptions: make([]ListedException, 0, len(r.Exceptions))}
	if r.Ratchet != nil {
		out.Ratchet = &ListedRatchet{
			HighWater: r.Ratchet.HighWater, DecidedBy: r.Ratchet.DecidedBy,
			DecidedOn: r.Ratchet.DecidedOn.String(), Reason: r.Ratchet.Reason,
		}
	}
	for i, e := range r.Exceptions {
		out.Exceptions = append(out.Exceptions, ListedException{
			Number: i + 1, Rule: e.Rule, Path: e.Path, Covers: e.EffectiveCovers(), Policy: e.Policy,
			DecidedBy: deref(e.DecidedBy), AddedByModel: deref(e.AddedByModel),
			DecidedOn: e.DecidedOn.String(), ReviewBy: e.ReviewBy.String(),
			Justification: e.Justification,
		})
	}
	return out
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// List renders every exception with its justification, so a vacuous one is read next to its
// neighbours (TOOLING-SPEC 6.3).
func (r Record) List() string {
	var out strings.Builder
	fmt.Fprintf(&out, "%d exceptions", len(r.Exceptions))
	if r.Ratchet != nil {
		fmt.Fprintf(&out, " (ratchet high_water %d, decided by %s)", r.Ratchet.HighWater, r.Ratchet.DecidedBy)
	}
	out.WriteString("\n")
	for i, e := range r.listing().Exceptions {
		fmt.Fprintf(&out, "\n#%d %s  %s  covers %s", i+1, e.Rule, e.Path, e.Covers)
		if e.Policy {
			out.WriteString("  policy")
		}
		out.WriteString("\n")
		writeProvenance(&out, e)
		fmt.Fprintf(&out, "    %s\n", e.Justification)
	}
	return out.String()
}

func writeProvenance(out *strings.Builder, e ListedException) {
	var parts []string
	for _, field := range []struct{ name, value string }{
		{"decided_by", e.DecidedBy},
		{"added_by_model", e.AddedByModel},
		{"decided_on", e.DecidedOn},
		{"review_by", e.ReviewBy},
	} {
		if field.value != "" {
			parts = append(parts, field.name+" "+field.value)
		}
	}
	if len(parts) > 0 {
		fmt.Fprintf(out, "    %s\n", strings.Join(parts, ", "))
	}
}

// ListJSON is List as JSON: {"ratchet": {...}, "exceptions": [...]}.
func (r Record) ListJSON() ([]byte, error) {
	data, err := json.MarshalIndent(r.listing(), "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode project record: %w", err)
	}
	return data, nil
}

// Explain renders the exception numbered selector (1-based), or every exception whose rule is
// selector. It fails when nothing matches.
func (r Record) Explain(selector string) (string, error) {
	selector = strings.TrimSpace(selector)
	entries := r.listing().Exceptions
	var shown []ListedException
	if n, err := strconv.Atoi(selector); err == nil {
		if n < 1 || n > len(entries) {
			return "", fmt.Errorf("no exception #%d; the record has %d", n, len(entries))
		}
		shown = entries[n-1 : n]
	} else {
		for _, e := range entries {
			if selector != "" && e.Rule == selector {
				shown = append(shown, e)
			}
		}
		if len(shown) == 0 {
			return "", fmt.Errorf("no exception for rule %q; use a rule id or a number from `record list`", selector)
		}
	}
	var out strings.Builder
	for _, e := range shown {
		fmt.Fprintf(&out, "exception #%d\n  rule:    %s\n  path:    %s\n  covers:  %s\n", e.Number, e.Rule, e.Path, e.Covers)
		if e.Policy {
			out.WriteString("  policy:  true (a deliberately broad exception)\n")
		}
		writeProvenance(&out, e)
		fmt.Fprintf(&out, "  why:     %s\n", e.Justification)
	}
	return out.String(), nil
}

// Entries are the exceptions as the listing shows them, numbered from 1.
func (r Record) Entries() []ListedException {
	return r.listing().Exceptions
}
