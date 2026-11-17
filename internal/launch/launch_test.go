package launch_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/launch"
)

func TestFindResolvesABareNameToAnAbsolutePath(t *testing.T) {
	path, err := launch.Find("go")
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(path) || filepath.Base(path) != "go" {
		t.Errorf("Find(go) = %q", path)
	}
}

func TestFindRefusesAnythingButABareName(t *testing.T) {
	for _, name := range []string{"", " ", "./go", "bin/go", "/usr/bin/go", "../go"} {
		if path, err := launch.Find(name); err == nil {
			t.Errorf("Find(%q) = %q, want an error", name, path)
		}
	}
}

func TestFindOfAMissingProgramIsNotFound(t *testing.T) {
	_, err := launch.Find("gqaci-no-such-program")
	if !errors.Is(err, exec.ErrNotFound) {
		t.Errorf("err = %v, want exec.ErrNotFound", err)
	}
}

func TestCommandRunsAnArgumentVectorWithoutAShell(t *testing.T) {
	cmd, err := launch.Command(t.Context(), "go", "env", "GOOS")
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(cmd.Path) || cmd.Args[0] != cmd.Path || strings.Join(cmd.Args[1:], " ") != "env GOOS" {
		t.Errorf("path %q, args %q", cmd.Path, cmd.Args)
	}
	out, err := cmd.Output()
	if err != nil || strings.TrimSpace(string(out)) == "" {
		t.Errorf("go env GOOS: %q, %v", out, err)
	}
}

func TestCommandPassesShellTextThroughAsOneArgument(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd, err := launch.Command(t.Context(), exe, "-test.run=^$", "; echo injected")
	if err != nil {
		t.Fatal(err)
	}
	if len(cmd.Args) != 3 || cmd.Args[2] != "; echo injected" {
		t.Errorf("args %q", cmd.Args)
	}
}

func TestCommandIsBoundToItsContext(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cmd, err := launch.Command(ctx, "go", "version")
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	if err := cmd.Run(); err == nil {
		t.Error("a command whose context was cancelled ran")
	}
}

func TestCommandRefusesARelativePath(t *testing.T) {
	for _, program := range []string{"./tool", "scripts/tool.sh", ""} {
		if _, err := launch.Command(t.Context(), program); err == nil {
			t.Errorf("Command(%q) succeeded", program)
		}
	}
}

func TestCommandRefusesAShellGivenACommandString(t *testing.T) {
	for _, argv := range [][]string{
		{"sh", "-c", "echo hi"},
		{"bash", "-lc", "echo hi"},
		{"/bin/bash", "-e", "-c", "echo hi"},
		{"env", "FOO=1", "sh", "-c", "echo hi"},
	} {
		_, err := launch.Command(t.Context(), argv[0], argv[1:]...)
		if err == nil || !strings.Contains(err.Error(), "command string") {
			t.Errorf("Command(%q) = %v, want a refusal", argv, err)
		}
	}
}

func TestCommandAllowsAShellRunningAScriptFile(t *testing.T) {
	for _, argv := range [][]string{
		{"bash", "scripts/check.sh", "-c"},
		{"sh", "-e", "check.sh"},
		{"git", "-c", "user.name=t", "status"},
	} {
		if _, err := launch.Command(t.Context(), argv[0], argv[1:]...); err != nil {
			t.Errorf("Command(%q) = %v", argv, err)
		}
	}
}
