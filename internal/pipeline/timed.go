package pipeline

import (
	"fmt"
	"io"
	"strings"
	"time"
)

// Timed wraps an Executor so that every step prints a start line and an end line, with the
// step's result and wall time, as the run goes; a long run's log then shows where the time went.
type Timed struct {
	inner Executor
	out   io.Writer
	now   func() time.Time
	took  []stepTime
	err   error
}

type stepTime struct {
	name    string
	result  string
	elapsed time.Duration
}

// NewTimed wraps inner; the lines go to out and now is the clock.
func NewTimed(inner Executor, out io.Writer, now func() time.Time) *Timed {
	return &Timed{inner: inner, out: out, now: now}
}

// Run runs the step and reports its start and end.
func (t *Timed) Run(step Step) Outcome {
	t.say("go-qa: %s: start\n", step.Name)
	began := t.now()
	outcome := t.inner.Run(step)
	elapsed := t.now().Sub(began)
	result := "passed"
	switch outcome.Kind {
	case KindFailed:
		result = "FAILED"
	case KindSkipped:
		result = "skipped"
	case KindPassed:
	}
	t.say("go-qa: %s: %s in %s\n", step.Name, result, round(elapsed))
	t.took = append(t.took, stepTime{name: step.Name, result: result, elapsed: elapsed})
	return outcome
}

// Summary is a table of the time every step took, and their total.
func (t *Timed) Summary() string {
	var out strings.Builder
	var total time.Duration
	out.WriteString("\ngo-qa stage times:\n")
	for _, s := range t.took {
		fmt.Fprintf(&out, "  %-12s %-8s %s\n", s.name, s.result, round(s.elapsed))
		total += s.elapsed
	}
	fmt.Fprintf(&out, "  %-12s %-8s %s\n", "total", "", round(total))
	return out.String()
}

func round(d time.Duration) time.Duration { return d.Round(100 * time.Millisecond) }

// say writes one line, keeping the first write error for Err.
func (t *Timed) say(format string, args ...any) {
	if _, err := fmt.Fprintf(t.out, format, args...); err != nil && t.err == nil {
		t.err = err
	}
}

// Err is the first error writing a line, if any.
func (t *Timed) Err() error { return t.err }
