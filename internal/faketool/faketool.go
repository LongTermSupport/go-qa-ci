// Package faketool lets a test stand in for an external program the toolchain launches.
//
// Install puts a symbolic link named like the program, pointing at the test binary, first on PATH.
// When the link is executed, the test binary's TestMain (which calls Main) behaves as the program
// was told to: it records its arguments and working directory, prints the canned output and
// exits with the canned code. No shell and no extra executable file is involved.
package faketool

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const envPrefix = "GOQA_FAKETOOL_"

const (
	logMode     = 0o600
	failureCode = 127
)

// Spec is how a fake program behaves.
type Spec struct {
	// Stdout and Stderr are printed as given.
	Stdout string
	Stderr string
	// Code is the exit code.
	Code int
	// Log is the file every invocation appends one "dir<TAB>arguments" line to.
	Log string
}

func envName(name string) string {
	return envPrefix + strings.ToUpper(strings.ReplaceAll(name, "-", "_"))
}

// Install makes name a fake program for the rest of the test and returns the path of its
// invocation log.
func Install(t *testing.T, name string, spec Spec) (logPath string) {
	t.Helper()
	dir := t.TempDir()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if linkErr := os.Symlink(exe, filepath.Join(dir, name)); linkErr != nil {
		t.Fatal(linkErr)
	}
	spec.Log = filepath.Join(dir, name+".log")
	encoded, err := json.Marshal(spec)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(envName(name), string(encoded))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return spec.Log
}

// Calls returns the invocation log lines of an installed fake.
func Calls(t *testing.T, logPath string) []string {
	t.Helper()
	root, err := os.OpenRoot(filepath.Dir(logPath))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := root.Close(); closeErr != nil {
			t.Error(closeErr)
		}
	}()
	data, err := root.ReadFile(filepath.Base(logPath))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
}

// Main is called first by a package's TestMain: when the test binary was started as an installed
// fake it returns the exit status the fake must end with and true, and TestMain exits with it;
// otherwise it returns false and the tests run.
func Main() (code int, served bool) {
	name := filepath.Base(os.Args[0])
	encoded, ok := os.LookupEnv(envName(name))
	if !ok {
		return 0, false
	}
	code, err := act(encoded)
	if err != nil {
		// The stderr write is best effort: the exit code already says the fake could not behave.
		_, _ = fmt.Fprintf(os.Stderr, "faketool %s: %v\n", name, err)
		return failureCode, true
	}
	return code, true
}

// act records the invocation, prints the canned output and returns the canned exit code.
func act(encoded string) (code int, err error) {
	var spec Spec
	if err = json.Unmarshal([]byte(encoded), &spec); err != nil {
		return 0, fmt.Errorf("reading the spec: %w", err)
	}
	dir, err := os.Getwd()
	if err != nil {
		return 0, fmt.Errorf("working directory: %w", err)
	}
	file, err := os.OpenFile(spec.Log, os.O_APPEND|os.O_CREATE|os.O_WRONLY, logMode)
	if err != nil {
		return 0, fmt.Errorf("opening the log: %w", err)
	}
	_, writeErr := fmt.Fprintf(file, "%s\t%s\n", dir, strings.Join(os.Args[1:], " "))
	if err = errors.Join(writeErr, file.Close()); err != nil {
		return 0, fmt.Errorf("writing the log: %w", err)
	}
	if _, err = os.Stdout.WriteString(spec.Stdout); err != nil {
		return 0, fmt.Errorf("writing stdout: %w", err)
	}
	if _, err = os.Stderr.WriteString(spec.Stderr); err != nil {
		return 0, fmt.Errorf("writing stderr: %w", err)
	}
	return spec.Code, nil
}
