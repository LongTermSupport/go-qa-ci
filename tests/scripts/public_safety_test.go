package scripts_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const plantedTerm = "zzplanted-term"

// fixture is a repository plus a denylist kept outside it.
type fixture struct {
	repo     string
	denylist string
}

func scanScript(t *testing.T) string {
	t.Helper()
	path, err := filepath.Abs("../../scripts/public-safety-scan.sh")
	if err != nil {
		t.Fatal(err)
	}
	return path
}

// initRepo makes dir a repository with notes.txt staged. The command line is constant; the
// directory is the only input.
func initRepo(t *testing.T, dir string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "sh", "-ec",
		"git init -q && git config user.email dev@example.com && git config user.name 'Example Dev' && git add notes.txt")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("repository setup: %v\n%s", err, out)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// newFixture makes a repository whose second tracked-file line carries the planted term, and a
// denylist outside it whose third line holds the term as a pattern.
func newFixture(t *testing.T) fixture {
	t.Helper()
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	if err := os.Mkdir(repo, 0o700); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(repo, "notes.txt"), "first line\nhas "+plantedTerm+" here\n")
	initRepo(t, repo)
	denylist := filepath.Join(root, "denylist")
	writeFile(t, denylist, "# comment\n\n"+plantedTerm+"\n")
	return fixture{repo: repo, denylist: denylist}
}

// scan runs the scanner with a constant command line; the script path and denylist travel in the
// environment and the mode arguments follow as positional parameters.
func scan(t *testing.T, repo, denylist string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "sh", "-c", `exec "$SCAN_SCRIPT" "$@"`, "sh")
	cmd.Args = append(cmd.Args, args...)
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "GQACI_DENYLIST="+denylist, "SCAN_SCRIPT="+scanScript(t))
	out, err := cmd.CombinedOutput()
	if err == nil {
		return string(out), 0
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatal(err)
	}
	return string(out), exitErr.ExitCode()
}

func TestTreeScanFailsWithLocationNotTerm(t *testing.T) {
	f := newFixture(t)
	out, code := scan(t, f.repo, f.denylist, "--tree")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1\n%s", code, out)
	}
	if strings.Contains(out, plantedTerm) {
		t.Errorf("output republishes the term:\n%s", out)
	}
	if !strings.Contains(out, "denylist line 3 matched at notes.txt:2") {
		t.Errorf("output lacks the denylist line and file line number:\n%s", out)
	}
}

func TestTreeScanCleanPasses(t *testing.T) {
	f := newFixture(t)
	writeFile(t, f.denylist, "unrelated-term\n")
	if out, code := scan(t, f.repo, f.denylist, "--tree"); code != 0 {
		t.Fatalf("exit code = %d, want 0\n%s", code, out)
	}
}

func TestMessageScanFails(t *testing.T) {
	f := newFixture(t)
	msg := filepath.Join(t.TempDir(), "msg")
	writeFile(t, msg, "subject\n\nmentions "+plantedTerm+"\n")
	out, code := scan(t, f.repo, f.denylist, "--message", msg)
	if code != 1 || strings.Contains(out, plantedTerm) || !strings.Contains(out, "text line 3") {
		t.Fatalf("code = %d, output:\n%s", code, out)
	}
}

func TestMissingDenylistRefused(t *testing.T) {
	f := newFixture(t)
	absent := filepath.Join(filepath.Dir(f.denylist), "absent")
	if _, code := scan(t, f.repo, absent); code != 2 {
		t.Errorf("missing denylist: exit code = %d, want 2", code)
	}
}

func TestDenylistInsideRepoRefused(t *testing.T) {
	f := newFixture(t)
	inside := filepath.Join(f.repo, "list")
	writeFile(t, inside, plantedTerm+"\n")
	if _, code := scan(t, f.repo, inside); code != 2 {
		t.Errorf("denylist inside repo: exit code = %d, want 2", code)
	}
}

func TestEmptyDenylistRefused(t *testing.T) {
	f := newFixture(t)
	writeFile(t, f.denylist, "# only a comment\n\n")
	if _, code := scan(t, f.repo, f.denylist); code != 2 {
		t.Errorf("empty denylist: exit code = %d, want 2", code)
	}
}
