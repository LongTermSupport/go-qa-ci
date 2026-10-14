// Package astgreptest lets a test binary stand in for the ast-grep executable, so the ast-grep
// detector's argument vector, exit-code handling and output parsing are tested without the real
// binary. It is the Go stand-in: no executable file is written with loosened permissions.
package astgreptest

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/astgrep"
)

const (
	outEnv  = "GQACI_FAKE_AST_GREP_OUT"
	argsEnv = "GQACI_FAKE_AST_GREP_ARGS"
	codeEnv = "GQACI_FAKE_AST_GREP_CODE"
	// scanOnlyEnv, when set, makes the stand-in exit 0 for the test subcommand.
	scanOnlyEnv = "GQACI_FAKE_AST_GREP_SCAN_ONLY"
	// failure is the exit status of the stand-in when it cannot do what it was configured to.
	failure = 70
)

// Serve is called first by a test binary's TestMain. A process started under the name ast-grep
// (see Install) records its arguments, prints the configured output and exits with the
// configured status; Serve does not return then.
func Serve() {
	if filepath.Base(os.Args[0]) == astgrep.Binary {
		os.Exit(standIn())
	}
}

func standIn() int {
	if recordArgs() != nil {
		return failure
	}
	if os.Getenv(scanOnlyEnv) != "" && len(os.Args) > 1 && os.Args[1] == "test" {
		return 0
	}
	if _, writeErr := os.Stdout.WriteString(os.Getenv(outEnv)); writeErr != nil {
		return failure
	}
	code, convErr := strconv.Atoi(os.Getenv(codeEnv))
	if convErr != nil {
		return failure
	}
	return code
}

// recordArgs writes the arguments of this invocation to the configured file.
func recordArgs() error {
	file := os.Getenv(argsEnv)
	handle, err := os.OpenRoot(filepath.Dir(file))
	if err != nil {
		return fmt.Errorf("opening the recording directory: %w", err)
	}
	writeErr := handle.WriteFile(filepath.Base(file), []byte(strings.Join(os.Args[1:], "\n")), 0o600)
	return errors.Join(writeErr, handle.Close())
}

// Install makes the test binary the only ast-grep on PATH for the test: a symlink named ast-grep
// that prints stdoutText and exits with code. It returns the file the arguments of the last
// invocation are recorded in, one per line.
func Install(t *testing.T, stdoutText string, code int) (argsFile string) {
	t.Helper()
	return install(t, stdoutText, code, "")
}

// InstallScanOnly is Install for a stand-in whose test subcommand always passes, so only the
// scans are affected by code.
func InstallScanOnly(t *testing.T, stdoutText string, code int) (argsFile string) {
	t.Helper()
	return install(t, stdoutText, code, "1")
}

func install(t *testing.T, stdoutText string, code int, scanOnly string) (argsFile string) {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.Symlink(self, filepath.Join(dir, astgrep.Binary)); err != nil {
		t.Fatal(err)
	}
	argsFile = filepath.Join(dir, "args")
	t.Setenv("PATH", dir)
	t.Setenv(outEnv, stdoutText)
	t.Setenv(argsEnv, argsFile)
	t.Setenv(codeEnv, strconv.Itoa(code))
	t.Setenv(scanOnlyEnv, scanOnly)
	return argsFile
}

// Args reads the arguments the stand-in recorded.
func Args(t *testing.T, argsFile string) []string {
	t.Helper()
	handle, err := os.OpenRoot(filepath.Dir(argsFile))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := handle.Close(); closeErr != nil {
			t.Error(closeErr)
		}
	}()
	text, err := handle.ReadFile(filepath.Base(argsFile))
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(string(text), "\n")
}
