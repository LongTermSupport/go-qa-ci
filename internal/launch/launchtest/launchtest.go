// Package launchtest starts programs from tests through package launch, so a test that runs git,
// go, a script or the built test binary is held to the same contract as the code it tests: an
// argument vector, a program found on PATH or given as an absolute path, and the test's context.
package launchtest

import (
	"os/exec"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/launch"
)

// Command is launch.Command bound to the test's context. A launch the contract refuses, or a
// program that is not installed, fails the test.
func Command(tb testing.TB, program string, args ...string) *exec.Cmd {
	tb.Helper()
	cmd, err := launch.Command(tb.Context(), program, args...)
	if err != nil {
		tb.Fatal(err)
		return nil
	}
	return cmd
}
