package tooltest_test

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/tooltest"
)

func TestAbsenceIsAllowedOnlyWhenTheOptOutNamesTheTool(t *testing.T) {
	for name, tc := range map[string]struct {
		optOut string
		want   bool
	}{
		"unset":                {"", false},
		"the tool":             {"ast-grep", true},
		"among others":         {"gremlins, ast-grep", true},
		"another tool":         {"gremlins", false},
		"a prefix of the tool": {"ast", false},
		"everything":           {"all", true},
		"1 is not a tool":      {"1", false},
	} {
		t.Run(name, func(t *testing.T) {
			if got := tooltest.AbsenceAllowed("ast-grep", tc.optOut); got != tc.want {
				t.Errorf("AbsenceAllowed(ast-grep, %q) = %v, want %v", tc.optOut, got, tc.want)
			}
		})
	}
}

func TestPinnedReadsTheToolLock(t *testing.T) {
	for _, tool := range []string{"ast-grep", "gremlins", "golangci-lint"} {
		pinned, err := tooltest.Pinned(tool)
		if err != nil || !pinned {
			t.Errorf("%s: pinned %v, err %v", tool, pinned, err)
		}
	}
	pinned, err := tooltest.Pinned("not-a-pinned-tool")
	if err != nil || pinned {
		t.Errorf("an unpinned tool: pinned %v, err %v", pinned, err)
	}
}

func TestRequirePassesWhenTheToolWasFound(t *testing.T) {
	tooltest.Require(t, "ast-grep", nil)
}

// TestRequireHelper is run in a child process by the tests below; on its own it does nothing.
func TestRequireHelper(t *testing.T) {
	tool := os.Getenv("TOOLTEST_HELPER_TOOL")
	if tool == "" {
		t.Skip("run only by the parent test")
	}
	tooltest.Require(t, tool, errors.New("not found on PATH"))
}

// requireIn runs TestRequireHelper in a child process with the given environment.
func requireIn(t *testing.T, tool, optOut string) (output string, failed bool) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(t.Context(), exe, "-test.run=^TestRequireHelper$", "-test.v")
	cmd.Env = append(os.Environ(), "TOOLTEST_HELPER_TOOL="+tool, tooltest.OptOutEnv+"="+optOut)
	out, runErr := cmd.CombinedOutput()
	var exitErr *exec.ExitError
	if runErr != nil && !errors.As(runErr, &exitErr) {
		t.Fatal(runErr)
	}
	return string(out), runErr != nil
}

func TestAPinnedToolThatIsAbsentFailsTheTest(t *testing.T) {
	out, failed := requireIn(t, "ast-grep", "")
	if !failed || !strings.Contains(out, "scripts/install-tools.sh ast-grep") || !strings.Contains(out, tooltest.OptOutEnv) {
		t.Errorf("failed %v, output:\n%s", failed, out)
	}
}

func TestTheOptOutSkipsTheTestAndSaysSo(t *testing.T) {
	out, failed := requireIn(t, "ast-grep", "ast-grep")
	if failed || !strings.Contains(out, "SKIP") || !strings.Contains(out, "opts out") {
		t.Errorf("failed %v, output:\n%s", failed, out)
	}
}

func TestAnUnpinnedToolFailsEvenWithTheOptOut(t *testing.T) {
	out, failed := requireIn(t, "not-a-pinned-tool", "all")
	if !failed || !strings.Contains(out, "tools.lock does not pin") {
		t.Errorf("failed %v, output:\n%s", failed, out)
	}
}
