package date_test

import (
	"testing"
	"time"

	"github.com/LongTermSupport/go-qa-ci/internal/date"
)

func TestParseAndString(t *testing.T) {
	d, err := date.Parse("2026-10-01")
	if err != nil {
		t.Fatal(err)
	}
	if d.IsZero() || d.String() != "2026-10-01" {
		t.Errorf("got %q zero=%v", d.String(), d.IsZero())
	}
}

func TestParseRejectsMalformed(t *testing.T) {
	for _, in := range []string{"", "next month", "2026-13-01", "2026-10-1", "2026-10-01T00:00:00Z", " 2026-10-01"} {
		if _, err := date.Parse(in); err == nil {
			t.Errorf("Parse(%q) succeeded", in)
		}
	}
}

func mustParse(t *testing.T, s string) date.Date {
	t.Helper()
	d, err := date.Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestZeroValueIsUnset(t *testing.T) {
	var d date.Date
	if !d.IsZero() || d.String() != "" {
		t.Errorf("zero date: %q zero=%v", d.String(), d.IsZero())
	}
}

func TestEqual(t *testing.T) {
	a, b, c := mustParse(t, "2026-10-01"), mustParse(t, "2026-10-01"), mustParse(t, "2026-10-02")
	if !a.Equal(b) || a.Equal(c) {
		t.Error("Equal wrong")
	}
}

func TestOrderingAndArithmetic(t *testing.T) {
	a, b := mustParse(t, "2026-02-27"), mustParse(t, "2026-03-02")
	if !a.Before(b) || b.Before(a) || !b.After(a) || a.Compare(b) >= 0 || b.Compare(a) <= 0 || a.Compare(a.AddDays(0)) != 0 {
		t.Error("ordering wrong")
	}
	if got := a.AddDays(3).String(); got != "2026-03-02" {
		t.Errorf("AddDays(3) = %s", got)
	}
	if got := b.AddDays(-3).String(); got != "2026-02-27" {
		t.Errorf("AddDays(-3) = %s", got)
	}
	if got := a.DaysUntil(b); got != 3 {
		t.Errorf("DaysUntil = %d", got)
	}
	if got := b.DaysUntil(a); got != -3 {
		t.Errorf("DaysUntil back = %d", got)
	}
}

func TestFromTimeUsesTheUTCDay(t *testing.T) {
	zone := time.FixedZone("east", 10*60*60)
	got := date.FromTime(time.Date(2026, 10, 8, 3, 0, 0, 0, zone))
	if got.String() != "2026-10-07" {
		t.Errorf("FromTime = %s, want 2026-10-07", got)
	}
}
