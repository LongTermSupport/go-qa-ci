package gotest_test

import (
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/gotest"
)

func TestRerunFlagFindsEveryFormOfCountAndShuffle(t *testing.T) {
	for _, args := range [][]string{{"-count=1"}, {"--count", "1"}, {"-shuffle=off"}, {"./...", "-args", "-test.count=3"}, {"-test.shuffle", "on"}} {
		if _, found := gotest.RerunFlag(args); !found {
			t.Errorf("%q: not found", args)
		}
	}
	for _, args := range [][]string{{"-race", "./..."}, {"-run", "count"}, {"-coverprofile=count.out"}} {
		if flag, found := gotest.RerunFlag(args); found {
			t.Errorf("%q: found %q", args, flag)
		}
	}
}
