package runners

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// slowestShown is how many packages and tests the timing report lists.
const slowestShown = 10

// testEvent is the part of a `go test -json` event the gate reads.
type testEvent struct {
	Action  string
	Package string
	Test    string
	Elapsed float64
	Output  string
}

// Timing is the wall time of one package or test.
type Timing struct {
	Name    string
	Elapsed float64
}

// Timings are the elapsed times of the packages and tests a go test run finished.
type Timings struct {
	Packages []Timing
	Tests    []Timing
}

// Parsed is a go test -json stream read back: the plain text and the timings.
type Parsed struct {
	Text    string
	Timings Timings
}

// ParseTestJSON turns the stdout of `go test -json` into the plain text go test would have
// printed (the output events, in order; a line that is not an event is kept as it is) and the
// timings of every package and test that passed or failed.
func ParseTestJSON(stdout string) Parsed {
	var timings Timings
	var plain strings.Builder
	for line := range strings.Lines(stdout) {
		var event testEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil || event.Action == "" {
			plain.WriteString(line)
			continue
		}
		switch event.Action {
		case "output":
			plain.WriteString(event.Output)
		case "pass", "fail":
			if event.Test == "" {
				timings.Packages = append(timings.Packages, Timing{event.Package, event.Elapsed})
			} else {
				timings.Tests = append(timings.Tests, Timing{event.Package + "." + event.Test, event.Elapsed})
			}
		}
	}
	return Parsed{Text: plain.String(), Timings: timings}
}

// add appends the timings of another batch.
func (t *Timings) add(other Timings) {
	t.Packages = append(t.Packages, other.Packages...)
	t.Tests = append(t.Tests, other.Tests...)
}

// Slowest renders the limit slowest packages and tests, longest first; it is empty when nothing
// was timed.
func (t Timings) Slowest(limit int) string {
	if len(t.Packages) == 0 && len(t.Tests) == 0 {
		return ""
	}
	var out strings.Builder
	list := func(title string, timings []Timing) {
		sorted := append([]Timing(nil), timings...)
		sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Elapsed > sorted[j].Elapsed })
		fmt.Fprintf(&out, "go-qa: test: %d slowest %s\n", min(limit, len(sorted)), title)
		for _, timing := range sorted[:min(limit, len(sorted))] {
			fmt.Fprintf(&out, "  %8.2fs  %s\n", timing.Elapsed, timing.Name)
		}
	}
	list("packages", t.Packages)
	list("tests", t.Tests)
	return out.String()
}
