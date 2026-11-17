package astgreptest_test

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"slices"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/astgrep"
	"github.com/LongTermSupport/go-qa-ci/internal/astgreptest"
	"github.com/LongTermSupport/go-qa-ci/internal/launch/launchtest"
)

func TestMain(m *testing.M) {
	if code, served := astgreptest.Serve(); served {
		os.Exit(code)
	}
	os.Exit(m.Run())
}

func TestInstalledStandInRecordsArgumentsPrintsOutputAndExitsWithTheCode(t *testing.T) {
	argsFile := astgreptest.Install(t, "[]", 1)
	var stdout bytes.Buffer
	cmd := launchtest.Command(t, astgrep.Binary, "scan", "--json=compact")
	cmd.Stdout = &stdout
	runErr := cmd.Run()
	var exitErr *exec.ExitError
	if !errors.As(runErr, &exitErr) || exitErr.ExitCode() != 1 {
		t.Fatalf("run error = %v", runErr)
	}
	if stdout.String() != "[]" {
		t.Errorf("stdout = %q", stdout.String())
	}
	if got := astgreptest.Args(t, argsFile); !slices.Equal(got, []string{"scan", "--json=compact"}) {
		t.Errorf("args = %q", got)
	}
}

func TestScanOnlyStandInLetsTheRuleTestsPass(t *testing.T) {
	astgreptest.InstallScanOnly(t, "[]", 1)
	if err := launchtest.Command(t, astgrep.Binary, "test").Run(); err != nil {
		t.Errorf("test subcommand failed: %v", err)
	}
	scan := launchtest.Command(t, astgrep.Binary, "scan")
	var exitErr *exec.ExitError
	if runErr := scan.Run(); !errors.As(runErr, &exitErr) || exitErr.ExitCode() != 1 {
		t.Errorf("scan run error = %v", runErr)
	}
}
