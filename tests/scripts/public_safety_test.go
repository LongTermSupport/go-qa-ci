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

// scanOptions varies one scanner invocation.
type scanOptions struct {
	env   []string // extra environment entries
	stdin string
}

// withoutDenylistEnv drops any inherited denylist variable so only the options decide the route.
func withoutDenylistEnv() []string {
	var kept []string
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "GQACI_DENYLIST=") {
			kept = append(kept, entry)
		}
	}
	return kept
}

func scan(t *testing.T, repo, denylist string, args ...string) (string, int) {
	t.Helper()
	return scanWith(t, repo, scanOptions{env: []string{"GQACI_DENYLIST=" + denylist}}, args...)
}

// git runs git in dir; every call site passes constant arguments.
func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "sh", "-ec", `exec git "$@"`, "sh")
	cmd.Args = append(cmd.Args, args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// assertLeaksNothing fails unless the scan exited 1 with a location and without the term.
func assertLeaksNothing(t *testing.T, out string, code int, location string) {
	t.Helper()
	if code != 1 {
		t.Fatalf("exit code = %d, want 1\n%s", code, out)
	}
	if strings.Contains(out, plantedTerm) {
		t.Errorf("output republishes the term:\n%s", out)
	}
	if !strings.Contains(out, location) {
		t.Errorf("output lacks %q:\n%s", location, out)
	}
}

// scanWith runs the scanner with a constant command line; the script path travels in the
// environment and the mode arguments follow as positional parameters.
func scanWith(t *testing.T, repo string, opts scanOptions, args ...string) (string, int) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "sh", "-c", `exec "$SCAN_SCRIPT" "$@"`, "sh")
	cmd.Args = append(cmd.Args, args...)
	cmd.Dir = repo
	cmd.Env = append(withoutDenylistEnv(), "SCAN_SCRIPT="+scanScript(t))
	cmd.Env = append(cmd.Env, opts.env...)
	cmd.Stdin = strings.NewReader(opts.stdin)
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

func TestInvalidPatternRefused(t *testing.T) {
	f := newFixture(t)
	writeFile(t, f.denylist, "foo(\n")
	out, code := scan(t, f.repo, f.denylist, "--tree")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2\n%s", code, out)
	}
	if strings.Contains(out, "clean") || !strings.Contains(out, "denylist line 1") {
		t.Errorf("output must name the denylist line and not claim clean:\n%s", out)
	}
}

func TestPathMatchReportsIndexNotPath(t *testing.T) {
	f := newFixture(t)
	named := plantedTerm + ".txt"
	writeFile(t, filepath.Join(f.repo, named), "innocent\n")
	git(t, f.repo, "add", named)
	writeFile(t, f.denylist, "zzplanted-term\\.txt\n")
	out, code := scan(t, f.repo, f.denylist, "--tree")
	assertLeaksNothing(t, out, code, "tracked path #")
	if strings.Contains(out, named) {
		t.Errorf("output names the matching path:\n%s", out)
	}
}

func TestDenylistInsideRepoViaSymlinkRefused(t *testing.T) {
	f := newFixture(t)
	writeFile(t, filepath.Join(f.repo, "inlist"), plantedTerm+"\n")
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(f.repo, link); err != nil {
		t.Fatal(err)
	}
	if out, code := scan(t, f.repo, filepath.Join(link, "inlist")); code != 2 {
		t.Errorf("denylist via symlink into repo: exit code = %d, want 2\n%s", code, out)
	}
}

func TestGitConfigDenylistRoute(t *testing.T) {
	f := newFixture(t)
	git(t, f.repo, "config", "gqaci.denylist", f.denylist)
	out, code := scanWith(t, f.repo, scanOptions{}, "--tree")
	assertLeaksNothing(t, out, code, "denylist line 3 matched at notes.txt:2")
}

func TestStagedScanFails(t *testing.T) {
	f := newFixture(t)
	out, code := scan(t, f.repo, f.denylist, "--staged")
	assertLeaksNothing(t, out, code, "staged text line")
}

func TestStdinScanFails(t *testing.T) {
	f := newFixture(t)
	opts := scanOptions{env: []string{"GQACI_DENYLIST=" + f.denylist}, stdin: "issue\nabout " + plantedTerm + "\n"}
	out, code := scanWith(t, f.repo, opts, "--stdin")
	assertLeaksNothing(t, out, code, "stdin text line 2")
}

func TestHistoryScanCoversIdentitiesTagsAndRefNames(t *testing.T) {
	cases := map[string]func(t *testing.T, repo string){
		"author identity": func(t *testing.T, repo string) {
			t.Helper()
			writeFile(t, filepath.Join(repo, "more.txt"), "x\n")
			git(t, repo, "add", "more.txt")
			git(t, repo, "-c", "user.name="+plantedTerm, "commit", "-q", "-m", "second")
		},
		"annotated tag message": func(t *testing.T, repo string) {
			t.Helper()
			git(t, repo, "tag", "-a", "v0", "-m", "release "+plantedTerm)
		},
		"ref name": func(t *testing.T, repo string) {
			t.Helper()
			git(t, repo, "branch", "feature-"+plantedTerm)
		},
	}
	for name, plant := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			writeFile(t, filepath.Join(f.repo, "notes.txt"), "first line\n")
			git(t, f.repo, "add", "notes.txt")
			git(t, f.repo, "commit", "-q", "-m", "clean subject")
			if out, code := scan(t, f.repo, f.denylist, "--history"); code != 0 {
				t.Fatalf("clean history: exit code = %d\n%s", code, out)
			}
			plant(t, f.repo)
			out, code := scan(t, f.repo, f.denylist, "--history")
			assertLeaksNothing(t, out, code, "history text line")
		})
	}
}

func TestEmptyDenylistRefused(t *testing.T) {
	f := newFixture(t)
	writeFile(t, f.denylist, "# only a comment\n\n")
	if _, code := scan(t, f.repo, f.denylist); code != 2 {
		t.Errorf("empty denylist: exit code = %d, want 2", code)
	}
}
