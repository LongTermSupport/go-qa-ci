package clean

import (
	"testing"
	"time"
)

func TestInjectedClock(t *testing.T) {
	fixed := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	now := func() time.Time { return fixed }
	if got := Age(now, fixed.Add(-time.Hour)); got != time.Hour {
		t.Fatal(got)
	}
	_ = time.Duration(5) * time.Second
}
