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

// Compare returns -1, 0 or +1 as d is before, the same day as, or after other.
func (d Date) Compare(other Date) int { return d.t.Compare(other.t) }

// Before reports whether d is an earlier day than other.
func (d Date) Before(other Date) bool { return d.Compare(other) < 0 }

// After reports whether d is a later day than other.
func (d Date) After(other Date) bool { return d.Compare(other) > 0 }

// AddDays returns the date n days later (earlier when n is negative).
func (d Date) AddDays(n int) Date { return Date{t: d.t.AddDate(0, 0, n)} }

// DaysUntil is the number of days from d to other; negative when other is earlier.
func (d Date) DaysUntil(other Date) int {
	return int(other.t.Sub(d.t).Hours() / hoursPerDay)
}

// FromTime is the UTC calendar date of t.
func FromTime(t time.Time) Date {
	y, m, day := t.UTC().Date()
	return Date{t: time.Date(y, m, day, 0, 0, 0, 0, time.UTC)}
}

const hoursPerDay = 24

// Today is the current UTC date; it is the only place the toolchain reads the clock.
func Today() Date { return FromTime(time.Now()) }
