// Package tooltest is how a test that needs an external tool treats its absence: as a failure.
//
// A gate that cannot run is not a pass, and the same holds for the tests that prove the gate. A
// test that needs a tool pinned in tools.lock fails when the tool is not installed, naming the
// command that installs it. The only way around is an explicit opt-out in the environment, which
// skips the test and says why.
package tooltest

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// OptOutEnv names the tools a run accepts to be without, comma separated, or "all". It exists for
// a developer machine that has not installed the pinned tools; CI sets nothing.
const OptOutEnv = "GQACI_TEST_WITHOUT_TOOLS"

// AbsenceAllowed reports whether the opt-out list names tool (or everything).
func AbsenceAllowed(tool, optOut string) bool {
	for name := range strings.SplitSeq(optOut, ",") {
		name = strings.TrimSpace(name)
		if name == "all" || name == tool {
			return true
		}
	}
	return false
}

// lockPath is tools.lock at the root of the module this package lives in.
func lockPath() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("cannot locate the source of the tooltest package")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "tools.lock"), nil
}

// Pinned reports whether tools.lock pins tool in any section.
func Pinned(tool string) (bool, error) {
	path, err := lockPath()
	if err != nil {
		return false, err
	}
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return false, fmt.Errorf("opening the module root: %w", err)
	}
	defer func() {
		if closeErr := root.Close(); closeErr != nil {
			fmt.Fprintf(os.Stderr, "tooltest: closing %s: %v\n", root.Name(), closeErr)
		}
	}()
	handle, err := root.Open(filepath.Base(path))
	if err != nil {
		return false, fmt.Errorf("opening tools.lock: %w", err)
	}
	defer func() {
		if closeErr := handle.Close(); closeErr != nil {
			fmt.Fprintf(os.Stderr, "tooltest: closing tools.lock: %v\n", closeErr)
		}
	}()
	scanner := bufio.NewScanner(handle)
	for scanner.Scan() {
		name, _, found := strings.Cut(scanner.Text(), " = ")
		if found && strings.TrimSpace(name) == tool && !strings.HasPrefix(name, "#") {
			return true, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return false, fmt.Errorf("reading tools.lock: %w", err)
	}
	return false, nil
}

// Require is called with the result of looking a tool up. When the tool was not found, the test
// fails (the tool is pinned, so it is a required dependency), unless OptOutEnv names it, which
// skips the test and says so. A tool tools.lock does not pin fails the test too: pin it first.
func Require(tb testing.TB, tool string, lookErr error) {
	tb.Helper()
	if lookErr == nil {
		return
	}
	pinned, err := Pinned(tool)
	if err != nil {
		tb.Fatalf("checking whether %s is pinned: %v", tool, err)
	}
	if !pinned {
		tb.Fatalf("this test needs %s, which tools.lock does not pin: pin it so its absence is a failure (%v)", tool, lookErr)
	}
	if AbsenceAllowed(tool, os.Getenv(OptOutEnv)) {
		tb.Skipf("%s is not installed and %s opts out of it: %v", tool, OptOutEnv, lookErr)
	}
	tb.Fatalf("%s is not installed, and a test that cannot run is not a pass: run scripts/install-tools.sh %s and put the directory scripts/install-tools.sh --print-path %s prints on PATH (or set %s=%s to skip it knowingly): %v",
		tool, tool, tool, OptOutEnv, tool, lookErr)
}
