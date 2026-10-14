package runners_test

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/config"
	"github.com/LongTermSupport/go-qa-ci/internal/faketool"
	"github.com/LongTermSupport/go-qa-ci/internal/runners"
)

func TestMutantsArgsPerMode(t *testing.T) {
	tests := []struct {
		name string
		opts runners.MutantsOptions
		want []string
	}{
		{
			"diff",
			runners.MutantsOptions{Mode: config.MutantsDiff, Base: "origin/main", ThresholdEfficacy: 100},
			[]string{"unleash", "--threshold-efficacy", "100", "--diff", "origin/main"},
		},
		{
			"full",
			runners.MutantsOptions{Mode: config.MutantsFull, ThresholdEfficacy: 70},
			[]string{"unleash", "--threshold-efficacy", "70"},
		},
	}
	for _, tt := range tests {
		got, err := runners.MutantsArgs(tt.opts)
		if err != nil || !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s: got %v, %v", tt.name, got, err)
		}
	}
}

func TestMutantsArgsRejectsWhatCannotRun(t *testing.T) {
	for name, opts := range map[string]runners.MutantsOptions{
		"off":           {Mode: config.MutantsOff},
		"diff, no base": {Mode: config.MutantsDiff},
	} {
		if _, err := runners.MutantsArgs(opts); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func mutantsCall(t *testing.T) (passed bool, err error) {
	t.Helper()
	_, passed, err = runners.Mutants(ctx(), t.TempDir(), runners.MutantsOptions{Mode: config.MutantsFull, ThresholdEfficacy: 100})
	if err != nil {
		return false, fmt.Errorf("mutants: %w", err)
	}
	return passed, nil
}

func TestMutantsPassesAndFailsOnTheExitCode(t *testing.T) {
	log := faketool.Install(t, "gremlins", faketool.Spec{Stdout: "survivors\n", Code: 10})
	passed, err := mutantsCall(t)
	if err != nil || passed {
		t.Errorf("got %v, %v", passed, err)
	}
	if calls := faketool.Calls(t, log); len(calls) != 1 {
		t.Errorf("calls: %q", calls)
	}
	faketool.Install(t, "gremlins", faketool.Spec{})
	if passed, err := mutantsCall(t); err != nil || !passed {
		t.Errorf("got %v, %v", passed, err)
	}
}
