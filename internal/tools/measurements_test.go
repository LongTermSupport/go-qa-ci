package tools_test

import (
	"io"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/tools"
)

func TestNewExecutorSharesATestRunOnlyWhenBothStepsAreSelected(t *testing.T) {
	for name, tc := range map[string]struct {
		selected []string
		shared   bool
	}{
		"both":          {[]string{"fmt", "test", "coverage"}, true},
		"test only":     {[]string{"test"}, false},
		"coverage only": {[]string{"coverage"}, false},
		"neither":       {[]string{"golangci"}, false},
	} {
		t.Run(name, func(t *testing.T) {
			e := tools.NewExecutor(tools.Context{Selected: tc.selected}, io.Discard)
			if (e.Shared != nil) != tc.shared {
				t.Errorf("shared = %v, want %v", e.Shared != nil, tc.shared)
			}
		})
	}
}
