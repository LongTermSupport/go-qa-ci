// Package date is a calendar date without a time of day, as written in the project record.
package date

import (
	"fmt"
	"time"
)

const layout = time.DateOnly

// Date is a calendar date; the zero value means "not set".
type Date struct {
	t time.Time
}

// Parse reads YYYY-MM-DD and nothing else.
func Parse(s string) (Date, error) {
	t, err := time.Parse(layout, s)
	if err != nil {
		return Date{}, fmt.Errorf("invalid date %q (want YYYY-MM-DD): %w", s, err)
	}
	return Date{t: t}, nil
}

// IsZero reports whether the date is unset.
func (d Date) IsZero() bool { return d.t.IsZero() }

// String is YYYY-MM-DD, or empty for an unset date.
func (d Date) String() string {
	if d.IsZero() {
		return ""
	}
	return d.t.Format(layout)
}

// Equal reports whether both dates are the same day.
func (d Date) Equal(other Date) bool { return d.t.Equal(other.t) }
