package scripts_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gitleaksStub is a stand-in gitleaks that records its arguments (and, for a directory scan, the
// contents of the files it would scan) and exits with STUB_EXIT, so the wrapper's own behaviour is
// tested without the real binary.
const gitleaksStub = `#!/bin/sh
printf '%s\n' "$*" >> "$STUB_LOG"
if [ "$1" = dir ]; then
	for last; do :; done
	find "$last" -type f -exec cat {} + >> "$STUB_LOG"
fi
exit "${STUB_EXIT:-0}"
`

type secretRun struct {
	out  string
	code int
	log  string
}

// runSecretScan runs scripts/secret-scan.sh in repo. withStub puts the stub gitleaks first on PATH;
// without it PATH holds no gitleaks at all.
func runSecretScan(t *testing.T, repo string, withStub bool, stubExit string, args ...string) secretRun {
	t.Helper()
	script, err := filepath.Abs("../../scripts/secret-scan.sh")
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	logDir := t.TempDir()
	logPath := filepath.Join(logDir, "log")
	writeFile(t, logPath, "")
	path := bin // an empty directory: no gitleaks anywhere on PATH
	if withStub {
		install := exec.CommandContext(t.Context(), "sh", "-ec",
			`printf '%s' "$STUB_BODY" > "$1/gitleaks" && chmod 700 "$1/gitleaks"`, "sh")
		install.Args = append(install.Args, bin)
		install.Env = append(os.Environ(), "STUB_BODY="+gitleaksStub)
		if out, installErr := install.CombinedOutput(); installErr != nil {
			t.Fatalf("installing the stub: %v\n%s", installErr, out)
		}
		path = bin + ":/usr/bin:/bin"
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(t.Context(), "bash", "-c", `exec "$BASH_BIN" "$SCAN_SCRIPT" "$@"`, "bash")
	cmd.Args = append(cmd.Args, args...)
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "PATH="+path, "BASH_BIN="+bash, "SCAN_SCRIPT="+script, "STUB_LOG="+logPath, "STUB_EXIT="+stubExit)
	out, runErr := cmd.CombinedOutput()
	code := 0
	var exitErr *exec.ExitError
	if runErr != nil {
		if !errors.As(runErr, &exitErr) {
			t.Fatal(runErr)
		}
		code = exitErr.ExitCode()
	}
	root, err := os.OpenRoot(logDir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := root.Close(); closeErr != nil {
			t.Error(closeErr)
		}
	}()
	logged, err := root.ReadFile("log")
	if err != nil {
		t.Fatal(err)
	}
	return secretRun{out: string(out), code: code, log: string(logged)}
}

func TestSecretScanRefusesWithoutGitleaks(t *testing.T) {
	f := newFixture(t)
	run := runSecretScan(t, f.repo, false, "0", "--tree")
	if run.code != 2 || !strings.Contains(run.out, "gitleaks is not installed") {
		t.Fatalf("code = %d, output:\n%s", run.code, run.out)
	}
}

func TestSecretScanRefusesSuppressionFiles(t *testing.T) {
	for _, name := range []string{".gitleaks.toml", ".gitleaksignore"} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			writeFile(t, filepath.Join(f.repo, name), "x\n")
			run := runSecretScan(t, f.repo, true, "0", "--tree")
			if run.code != 2 || !strings.Contains(run.out, name+" is not allowed") {
				t.Fatalf("code = %d, output:\n%s", run.code, run.out)
			}
			if run.log != "" {
				t.Errorf("gitleaks must not run when a suppression file exists:\n%s", run.log)
			}
		})
	}
}

func TestSecretScanModesInvokeGitleaksWithoutSuppression(t *testing.T) {
	cases := map[string]struct {
		mode string
		want []string
	}{
		"history": {"--history", []string{"git ", "--log-opts=--all"}},
		"staged":  {"--staged", []string{"git ", "--pre-commit", "--staged"}},
		"tree":    {"--tree", []string{"dir "}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			run := runSecretScan(t, f.repo, true, "0", tc.mode)
			if run.code != 0 || !strings.Contains(run.out, "secret-scan: clean ("+tc.mode+")") {
				t.Fatalf("code = %d, output:\n%s", run.code, run.out)
			}
			for _, want := range append(tc.want, "--redact", "--ignore-gitleaks-allow", "--gitleaks-ignore-path") {
				if !strings.Contains(run.log, want) {
					t.Errorf("gitleaks arguments lack %q:\n%s", want, run.log)
				}
			}
		})
	}
}

func TestSecretScanDefaultModeIsHistory(t *testing.T) {
	f := newFixture(t)
	run := runSecretScan(t, f.repo, true, "0")
	if run.code != 0 || !strings.Contains(run.log, "--log-opts=--all") {
		t.Fatalf("code = %d, log:\n%s\noutput:\n%s", run.code, run.log, run.out)
	}
}

func TestSecretScanPropagatesFindingsAndNeverClaimsClean(t *testing.T) {
	f := newFixture(t)
	run := runSecretScan(t, f.repo, true, "1", "--tree")
	if run.code != 1 || strings.Contains(run.out, "clean") {
		t.Fatalf("code = %d, output:\n%s", run.code, run.out)
	}
}

func TestSecretScanRefusesUnknownMode(t *testing.T) {
	f := newFixture(t)
	run := runSecretScan(t, f.repo, true, "0", "--bogus")
	if run.code != 2 || run.log != "" {
		t.Fatalf("code = %d, log %q, output:\n%s", run.code, run.log, run.out)
	}
}

func TestSecretScanTreeDoesNotReadThroughSymlinkToOutsideFile(t *testing.T) {
	f := newFixture(t)
	outside := filepath.Join(t.TempDir(), "outside.txt")
	writeFile(t, outside, "outside-only-content\n")
	if err := os.Symlink(outside, filepath.Join(f.repo, "link")); err != nil {
		t.Fatal(err)
	}
	git(t, f.repo, "add", "link")
	run := runSecretScan(t, f.repo, true, "0", "--tree")
	if run.code != 0 {
		t.Fatalf("code = %d, output:\n%s", run.code, run.out)
	}
	if strings.Contains(run.log, "outside-only-content") {
		t.Errorf("a file outside the repository was scanned:\n%s", run.log)
	}
	if !strings.Contains(run.log, "has "+plantedTerm) {
		t.Errorf("tracked content was not scanned:\n%s", run.log)
	}
}
