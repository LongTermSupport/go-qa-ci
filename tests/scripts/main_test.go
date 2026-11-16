package scripts_test

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// stubName is the program the scripts tests stand in for. A test puts a symlink of that name to
// this test binary first on PATH (installStub), so no executable file is ever written: the binary
// is already executable, and TestMain acts as the stub when it starts under that name.
const stubName = "gitleaks"

// TestMain runs the tests, or, started as stubName, is the stand-in gitleaks.
func TestMain(m *testing.M) {
	if filepath.Base(os.Args[0]) == stubName {
		os.Exit(runGitleaksStub(os.Args[1:]))
	}
	os.Exit(m.Run())
}

// runGitleaksStub records its arguments on one line of the file STUB_LOG names, and for a
// directory scan (first argument "dir") the contents of every regular file under its last
// argument, in name order; it then exits with STUB_EXIT (0 when unset). Anything it cannot do is
// said on stderr with exit 125, which no test expects, so a broken stub fails the test using it.
func runGitleaksStub(args []string) int {
	if err := logStubRun(args); err != nil {
		fmt.Fprintf(os.Stderr, "gitleaks stub: %v\n", err)
		return 125
	}
	code, set := os.LookupEnv("STUB_EXIT")
	if !set {
		return 0
	}
	n, err := strconv.Atoi(code)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gitleaks stub: STUB_EXIT %q is not a number\n", code)
		return 125
	}
	return n
}

func logStubRun(args []string) error {
	logPath := os.Getenv("STUB_LOG")
	if logPath == "" {
		return fmt.Errorf("STUB_LOG is not set")
	}
	logDir, err := os.OpenRoot(filepath.Dir(logPath))
	if err != nil {
		return fmt.Errorf("opening the log's directory: %w", err)
	}
	defer closeQuietly(logDir)
	log, err := logDir.OpenFile(filepath.Base(logPath), os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		return fmt.Errorf("opening the log: %w", err)
	}
	text := strings.Join(args, " ") + "\n"
	if len(args) > 1 && args[0] == "dir" {
		scanned, scanErr := scannedContents(args[len(args)-1])
		if scanErr != nil {
			closeQuietly(log)
			return scanErr
		}
		text += scanned
	}
	if _, writeErr := log.WriteString(text); writeErr != nil {
		closeQuietly(log)
		return fmt.Errorf("writing the log: %w", writeErr)
	}
	if closeErr := log.Close(); closeErr != nil {
		return fmt.Errorf("closing the log: %w", closeErr)
	}
	return nil
}

// scannedContents is the contents of every regular file under dir, in name order.
func scannedContents(dir string) (string, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return "", fmt.Errorf("opening the scanned directory: %w", err)
	}
	defer closeQuietly(root)
	var out strings.Builder
	err = fs.WalkDir(root.FS(), ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		content, readErr := root.ReadFile(name)
		if readErr != nil {
			return fmt.Errorf("reading %s: %w", name, readErr)
		}
		out.Write(content)
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("walking the scanned directory: %w", err)
	}
	return out.String(), nil
}

// closeQuietly closes c on a path that is already returning an error, or after a read: the
// stub's verdict is its exit code, and a close error there changes nothing it reports.
func closeQuietly(c interface{ Close() error }) {
	if err := c.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "gitleaks stub: %v\n", err)
	}
}

// installStub puts the stand-in gitleaks in dir: a symlink named stubName to this test binary.
func installStub(t *testing.T, dir string) {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(self, filepath.Join(dir, stubName)); err != nil {
		t.Fatal(err)
	}
}
