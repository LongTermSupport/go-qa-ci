// Package record is the project record: the one place an exception to a defence is decided, each
// with a written justification (TOOLING-SPEC 6.1 to 6.3).
//
// File format (qa/record.toml by default):
//
//	[ratchet]                    # ceiling on the entry count
//	high_water = 1
//	decided_by = "example-owner"
//	decided_on = 2026-01-15
//	reason = "why the ceiling is at this value"
//
//	[[exception]]
//	rule = "errcheck"            # identifier exactly as the defence prints it
//	path = "internal/parse/x.go" # one file; a glob needs policy = true
//	justification = "the hazard accepted and the scope it is accepted in"
//	decided_by = "example-owner"
//	added_by_model = "human"     # or the model id that wrote the entry
//	decided_on = 2026-01-15
//	review_by = 2026-04-15
//	covers = "suppressions"      # or "findings"
//
// This package checks structure and wording; budgets, deadlines, trust tiers and the ratchet are
// a separate concern.
package record

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	toml "github.com/pelletier/go-toml/v2"

	"github.com/LongTermSupport/go-qa-ci/internal/date"
)

// Covers is what an exception licenses.
type Covers string

// The things an exception can license.
const (
	// CoversSuppressions licenses a suppression written in the source or configuration; the
	// suppression guard matches it. This is the default.
	CoversSuppressions Covers = "suppressions"
	// CoversFindings licenses the findings a detector that applies the record reports at the
	// path, with nothing written in the source.
	CoversFindings Covers = "findings"
)

// Record is the parsed project record.
type Record struct {
	// Ratchet is the signed ceiling on the number of exceptions; nil when absent.
	Ratchet *Ratchet
	// Exceptions are the decided exceptions in file order.
	Exceptions []Exception
}

// Ratchet is the committed ceiling on the entry count.
type Ratchet struct {
	// HighWater is the maximum number of exceptions.
	HighWater int
	// DecidedBy is the human who set the ceiling.
	DecidedBy string
	// DecidedOn is when the ceiling was last raised or adopted.
	DecidedOn date.Date
	// Reason says why the ceiling is at this value.
	Reason string
}

// Exception is one decided exception.
type Exception struct {
	// Rule is the identifier exactly as the defence prints it.
	Rule string
	// Path is a file, or a glob when Policy is set, relative to the project root.
	Path string
	// Justification names the hazard accepted and the scope of the exception.
	Justification string
	// DecidedBy is who decided it; nil when the file does not say.
	DecidedBy *string
	// AddedByModel is the model id that wrote the entry, or "human"; nil when absent.
	AddedByModel *string
	// DecidedOn is when it was decided; zero when absent.
	DecidedOn date.Date
	// ReviewBy is when it must be re-decided or removed; zero when absent.
	ReviewBy date.Date
	// Policy marks a deliberately broad (glob) exception.
	Policy bool
	// Covers is what the exception licenses.
	Covers Covers
	// Line is the 1-based line of the entry's header in the file; 0 for an entry not read from a file.
	Line int
}

// EffectiveCovers is Covers, with the unset value read as the default, CoversSuppressions.
func (e Exception) EffectiveCovers() Covers {
	if e.Covers == "" {
		return CoversSuppressions
	}
	return e.Covers
}

type rawRecord struct {
	Ratchet    *rawRatchet    `toml:"ratchet"`
	Exceptions []rawException `toml:"exception"`
}

type rawRatchet struct {
	HighWater int    `toml:"high_water"`
	DecidedBy string `toml:"decided_by"`
	DecidedOn any    `toml:"decided_on"`
	Reason    string `toml:"reason"`
}

type rawException struct {
	Rule          string  `toml:"rule"`
	Path          string  `toml:"path"`
	Justification string  `toml:"justification"`
	DecidedBy     *string `toml:"decided_by"`
	AddedByModel  *string `toml:"added_by_model"`
	DecidedOn     any     `toml:"decided_on"`
	ReviewBy      any     `toml:"review_by"`
	Policy        bool    `toml:"policy"`
	Covers        string  `toml:"covers"`
}

const exceptionHeader = "[[exception]]"

// Load reads the record at path. A missing file is an empty record; a file that cannot be read or
// parsed, has an unknown key, a malformed date or an unknown covers value is an error.
func Load(path string) (Record, error) {
	data, err := readFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Record{}, nil
	}
	if err != nil {
		return Record{}, fmt.Errorf("read project record: %w", err)
	}
	return Parse(data)
}

// readFile reads path through a root opened on its directory, so the read cannot leave it.
func readFile(path string) ([]byte, error) {
	dir, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	data, readErr := dir.ReadFile(filepath.Base(path))
	if closeErr := dir.Close(); readErr == nil && closeErr != nil {
		return nil, fmt.Errorf("close: %w", closeErr)
	}
	if readErr != nil {
		return nil, fmt.Errorf("read: %w", readErr)
	}
	return data, nil
}

// Parse decodes the contents of a record file.
func Parse(data []byte) (Record, error) {
	var raw rawRecord
	dec := toml.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&raw); err != nil {
		var strict *toml.StrictMissingError
		if errors.As(err, &strict) {
			keys := make([]string, 0, len(strict.Errors))
			for _, e := range strict.Errors {
				keys = append(keys, fmt.Sprintf("%q", strings.Join(e.Key(), ".")))
			}
			return Record{}, fmt.Errorf("unknown key %s", strings.Join(keys, ", "))
		}
		return Record{}, fmt.Errorf("invalid project record: %w", err)
	}
	return convert(raw, headerLines(data))
}

// headerLines returns the 1-based line of every [[exception]] header, in file order.
func headerLines(data []byte) []int {
	var lines []int
	for i, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == exceptionHeader {
			lines = append(lines, i+1)
		}
	}
	return lines
}

func convert(raw rawRecord, headers []int) (Record, error) {
	var rec Record
	if raw.Ratchet != nil {
		decidedOn, err := toDate(raw.Ratchet.DecidedOn)
		if err != nil {
			return Record{}, fmt.Errorf("ratchet.decided_on: %w", err)
		}
		rec.Ratchet = &Ratchet{
			HighWater: raw.Ratchet.HighWater,
			DecidedBy: raw.Ratchet.DecidedBy,
			DecidedOn: decidedOn,
			Reason:    raw.Ratchet.Reason,
		}
	}
	for i, r := range raw.Exceptions {
		label := fmt.Sprintf("exception #%d", i+1)
		decidedOn, err := toDate(r.DecidedOn)
		if err != nil {
			return Record{}, fmt.Errorf("%s decided_on: %w", label, err)
		}
		reviewBy, err := toDate(r.ReviewBy)
		if err != nil {
			return Record{}, fmt.Errorf("%s review_by: %w", label, err)
		}
		covers, err := parseCovers(r.Covers)
		if err != nil {
			return Record{}, fmt.Errorf("%s: %w", label, err)
		}
		line := 0
		if i < len(headers) {
			line = headers[i]
		}
		rec.Exceptions = append(rec.Exceptions, Exception{
			Rule: r.Rule, Path: r.Path, Justification: r.Justification,
			DecidedBy: r.DecidedBy, AddedByModel: r.AddedByModel,
			DecidedOn: decidedOn, ReviewBy: reviewBy,
			Policy: r.Policy, Covers: covers, Line: line,
		})
	}
	return rec, nil
}

// toDate accepts an absent value, a TOML local date, or a string holding YYYY-MM-DD.
func toDate(value any) (date.Date, error) {
	switch v := value.(type) {
	case nil:
		return date.Date{}, nil
	case toml.LocalDate:
		return parseDate(v.String())
	case string:
		return parseDate(v)
	default:
		return date.Date{}, fmt.Errorf("want a date (YYYY-MM-DD), found %T", value)
	}
}

func parseDate(s string) (date.Date, error) {
	d, err := date.Parse(s)
	if err != nil {
		return date.Date{}, fmt.Errorf("date: %w", err)
	}
	return d, nil
}

func parseCovers(s string) (Covers, error) {
	switch Covers(s) {
	case "", CoversSuppressions:
		return CoversSuppressions, nil
	case CoversFindings:
		return CoversFindings, nil
	default:
		return "", fmt.Errorf("unknown covers %q (want %q or %q)", s, CoversSuppressions, CoversFindings)
	}
}
