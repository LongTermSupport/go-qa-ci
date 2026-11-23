package pipeline_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/LongTermSupport/go-qa-ci/internal/pipeline"
)

// steppingClock advances by step on every reading.
func steppingClock(step time.Duration) func() time.Time {
	now := time.Unix(0, 0)
	return func() time.Time {
		now = now.Add(step)
		return now
	}
}

func TestTimedPrintsAStartAndAnEndLineWithTheResultAndElapsedTime(t *testing.T) {
	var out bytes.Buffer
	exec := pipeline.NewTimed(&fake{failing: []pipeline.Step{testStep}}, &out, steppingClock(1500*time.Millisecond))
	pipeline.Execute([]pipeline.Step{fmtStep, testStep}, exec, false)
	want := "go-qa: fmt: start\n" +
		"go-qa: fmt: passed in 1.5s\n" +
		"go-qa: test: start\n" +
		"go-qa: test: FAILED in 1.5s\n"
	if out.String() != want {
		t.Errorf("got:\n%s\nwant:\n%s", out.String(), want)
	}
}

func TestTimedSummaryListsEveryStageWithItsTime(t *testing.T) {
	var out bytes.Buffer
	exec := pipeline.NewTimed(&fake{}, &out, steppingClock(2*time.Second))
	pipeline.Execute([]pipeline.Step{fmtStep, testStep}, exec, false)
	summary := exec.Summary()
	for _, want := range []string{"stage times:", "fmt", "test", "2s", "total"} {
		if !strings.Contains(summary, want) {
			t.Errorf("summary lacks %q:\n%s", want, summary)
		}
	}
}
