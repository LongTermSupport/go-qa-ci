package runners_test

import (
	"fmt"
	"reflect"
	"strings"
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
			[]string{"unleash", "--config", "/cfg/gremlins.yaml", "--diff", "origin/main"},
		},
		{
			"full",
			runners.MutantsOptions{Mode: config.MutantsFull, ThresholdEfficacy: 70},
			[]string{"unleash", "--config", "/cfg/gremlins.yaml"},
		},
	}
	for _, tt := range tests {
		got, err := runners.MutantsArgs(tt.opts, "/cfg/gremlins.yaml")
		if err != nil || !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s: got %v, %v", tt.name, got, err)
		}
	}
}

// gremlins fails when efficacy <= threshold and reads 0 as no gate.
func TestMutantsConfigWritesTheThresholdInGremlinsOwnComparison(t *testing.T) {
	for threshold, want := range map[int]string{
		0:   "unleash:\n  timeout-coefficient: 10\n  threshold:\n    efficacy: 0\n",
		70:  "unleash:\n  timeout-coefficient: 10\n  threshold:\n    efficacy: 69.9999\n",
		100: "unleash:\n  timeout-coefficient: 10\n  threshold:\n    efficacy: 99.9999\n",
	} {
		if got := runners.MutantsConfig(threshold); got != want {
			t.Errorf("%d: got %q, want %q", threshold, got, want)
		}
	}
}

func TestMutantsArgsRejectsWhatCannotRun(t *testing.T) {
	for name, opts := range map[string]runners.MutantsOptions{
		"off":           {Mode: config.MutantsOff},
		"diff, no base": {Mode: config.MutantsDiff},
	} {
		if _, err := runners.MutantsArgs(opts, "/cfg/gremlins.yaml"); err == nil {
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
	calls := faketool.Calls(t, log)
	if len(calls) != 1 || !strings.Contains(calls[0], "unleash --config ") || strings.Contains(calls[0], "--threshold-efficacy") {
		t.Errorf("calls: %q", calls)
	}
	faketool.Install(t, "gremlins", faketool.Spec{})
	if passed, err := mutantsCall(t); err != nil || !passed {
		t.Errorf("got %v, %v", passed, err)
	}
}
