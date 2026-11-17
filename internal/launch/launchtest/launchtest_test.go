package launchtest_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/launch/launchtest"
)

func TestCommandIsALaunchBoundToTheTest(t *testing.T) {
	cmd := launchtest.Command(t, "go", "env", "GOOS")
	if !filepath.IsAbs(cmd.Path) {
		t.Errorf("path %q is not resolved", cmd.Path)
	}
	out, err := cmd.Output()
	if err != nil || strings.TrimSpace(string(out)) == "" {
		t.Errorf("go env GOOS: %q, %v", out, err)
	}
}

// recorder stands in for a test, to see that a refused launch fails it.
type recorder struct {
	testing.TB
	fatal string
}

func (r *recorder) Fatal(args ...any) {
	for _, arg := range args {
		if err, ok := arg.(error); ok {
			r.fatal = err.Error()
		}
	}
}

func TestARefusedLaunchFailsTheTest(t *testing.T) {
	r := &recorder{TB: t}
	if cmd := launchtest.Command(r, "./relative-tool"); cmd != nil {
		t.Errorf("a refused launch returned %v", cmd)
	}
	if !strings.Contains(r.fatal, "relative-tool") {
		t.Errorf("fatal = %q", r.fatal)
	}
}
