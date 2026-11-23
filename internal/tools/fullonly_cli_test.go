package tools_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/cli"
	"github.com/LongTermSupport/go-qa-ci/internal/faketool"
	"github.com/LongTermSupport/go-qa-ci/internal/launch/launchtest"
)

const fullOnlyConfig = `
[[stage]]
name = "cross-build"
phase = "runners"
says = "builds for another platform"
fix = "fix the build"
command = ["cross-build"]
when = "full"
`

func gitHere(t *testing.T, dir string, args ...string) {
	t.Helper()
	full := append([]string{"-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false", "-c", "init.defaultBranch=main"}, args...)
	cmd := launchtest.Command(t, "git", full...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func putFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	path := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// Defence for the class "a diff-mode run runs a stage that is for full runs only": through the
// real command line, a project stage marked when = "full" never runs under --changed, whether the
// diff is narrowed to a package or widened to the whole project by go.mod. Widening means test
// every package; it never means run the full-only stages.
func TestChangedRunsNeverRunAFullOnlyProjectStage(t *testing.T) {
	for name, change := range map[string]struct{ file, content string }{
		"narrowed by a package": {"leaf/leaf.go", "package leaf\n\nfunc Leaf() int { return 2 }\n"},
		"widened by go.mod":     {"go.mod", "module example.com/m\n\ngo 1.26\n\n// edited\n"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("CI", "true")
			log := faketool.Install(t, "cross-build", faketool.Spec{})
			dir := t.TempDir()
			putFile(t, dir, "go.mod", "module example.com/m\n\ngo 1.26\n")
			putFile(t, dir, "leaf/leaf.go", "package leaf\n\nfunc Leaf() int { return 1 }\n")
			putFile(t, dir, "qa/go-qa.toml", fullOnlyConfig)
			gitHere(t, dir, "init")
			gitHere(t, dir, "add", "-A")
			gitHere(t, dir, "commit", "-m", "initial")
			gitHere(t, dir, "checkout", "-b", "feature")
			putFile(t, dir, change.file, change.content)
			gitHere(t, dir, "commit", "-am", "change")
			var stdout, stderr bytes.Buffer
			code := cli.Run([]string{"--root", dir, "run", "--no-fix", "--changed", "--base", "main", "-t", "cross-build"}, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("exit %d\n%s\n%s", code, stdout.String(), stderr.String())
			}
			if calls := faketool.Calls(t, log); len(calls) != 0 {
				t.Errorf("the full-only stage ran in a --changed run: %q\n%s", calls, stdout.String())
			}
			if !strings.Contains(stdout.String(), "cross-build") || !strings.Contains(stdout.String(), "skipped") {
				t.Errorf("summary does not show the stage skipped:\n%s", stdout.String())
			}
		})
	}
}
