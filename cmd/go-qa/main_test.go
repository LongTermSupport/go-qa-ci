package main_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/cli"
	"github.com/LongTermSupport/go-qa-ci/internal/vet"
)

// buildGoQA compiles the command under test into a temporary directory.
func buildGoQA(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "go-qa")
	build := exec.CommandContext(t.Context(), "go", "build", "-o", binary, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	return binary
}

func runBinary(t *testing.T, binary, dir string, env []string, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	var out, errOut bytes.Buffer
	cmd := exec.CommandContext(t.Context(), binary, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	err = cmd.Run()
	return out.String(), errOut.String(), err
}

func TestWithoutTheMarkerTheBinaryIsTheCommandLine(t *testing.T) {
	binary := buildGoQA(t)
	out, _, err := runBinary(t, binary, t.TempDir(), []string{vet.EnvVar + "="}, "--version")
	if err != nil || strings.TrimSpace(out) != cli.Version {
		t.Errorf("got %q, %v", out, err)
	}
}

func TestWithTheMarkerTheBinaryIsAGoVetTool(t *testing.T) {
	binary := buildGoQA(t)
	marker := []string{vet.EnvVar + "=1"}
	out, stderr, err := runBinary(t, binary, t.TempDir(), marker, "-flags")
	if err != nil {
		t.Fatalf("-flags: %v\n%s", err, stderr)
	}
	var flags []map[string]any
	if jsonErr := json.Unmarshal([]byte(out), &flags); jsonErr != nil {
		t.Errorf("-flags is not JSON: %v\n%s", jsonErr, out)
	}

	module := t.TempDir()
	for name, text := range map[string]string{"go.mod": "module example.com/m\n\ngo 1.26\n", "m.go": "package m\n"} {
		if writeErr := os.WriteFile(filepath.Join(module, name), []byte(text), 0o600); writeErr != nil {
			t.Fatal(writeErr)
		}
	}
	_, stderr, err = runBinary(t, "go", module, marker, "vet", "-vettool="+binary, "-json", "./...")
	if err != nil {
		t.Errorf("go vet with go-qa as the tool: %v\n%s", err, stderr)
	}
}
