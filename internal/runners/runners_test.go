package runners_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/faketool"
)

func TestMain(m *testing.M) {
	faketool.Main()
	os.Exit(m.Run())
}

const fileMode = 0o600

// module writes a Go module named example.com/fixture into a temporary directory and returns it.
func module(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	if _, ok := files["go.mod"]; !ok {
		files["go.mod"] = "module example.com/fixture\n\ngo 1.26\n"
	}
	for name, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), fileMode); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func ctx() context.Context { return context.Background() }

func TestMissingProgramIsAnErrorNeverASkip(t *testing.T) {
	goPath, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}
	// Only the go tool is findable, so the program under test is the one that is missing.
	t.Setenv("PATH", filepath.Dir(goPath))
	calls := map[string]func() error{
		"govulncheck": func() error { return vulnCall(t) },
		"gremlins":    func() error { _, err := mutantsCall(t); return err },
		"deadcode":    func() error { return deadcodeCall(t) },
	}
	for name, call := range calls {
		if err := call(); err == nil || !strings.Contains(err.Error(), name+" not found on PATH") {
			t.Errorf("%s: got %v", name, err)
		}
	}
}
