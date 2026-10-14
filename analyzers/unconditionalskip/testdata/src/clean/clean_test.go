package clean

import (
	"os"
	"testing"
)

func TestSkipsOnACondition(t *testing.T) {
	if os.Getenv("NEEDS_THING") == "" {
		t.Skip("NEEDS_THING is not set")
	}
}

func TestSkipsInShortMode(t *testing.T) {
	if testing.Short() {
		t.Skipf("short mode")
	}
}

func TestSkipsInASwitch(t *testing.T) {
	switch os.Getenv("MODE") {
	case "x":
		t.SkipNow()
	}
}

func TestHasNoSkip(t *testing.T) {
	t.Log("runs")
}

// A helper may skip unconditionally: whether the test runs is its caller's decision.
func requireThing(t *testing.T) {
	t.Helper()
	t.Skip("no thing here")
}

type notTesting struct{}

func (notTesting) Skip(string) {}

func TestOtherSkip(t *testing.T) {
	var n notTesting
	n.Skip("not the testing method")
}
