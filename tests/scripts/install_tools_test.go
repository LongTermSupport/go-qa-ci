package scripts_test

import (
	"bufio"
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// lockEntry is one `name = "value"` line of tools.lock.
type lockEntry struct{ section, name, value string }

func readLock(t *testing.T) []lockEntry {
	t.Helper()
	root, err := os.OpenRoot(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := root.Close(); closeErr != nil {
			t.Error(closeErr)
		}
	}()
	data, err := root.ReadFile("tools.lock")
	if err != nil {
		t.Fatal(err)
	}
	var entries []lockEntry
	section := ""
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		switch {
		case line == "" || strings.HasPrefix(line, "#"):
		case strings.HasPrefix(line, "["):
			section = strings.Trim(line, "[]")
		default:
			name, value, ok := strings.Cut(line, " = ")
			if !ok {
				t.Fatalf("tools.lock: malformed line %q", line)
			}
			entries = append(entries, lockEntry{section, name, strings.Trim(value, `"`)})
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return entries
}

func lockValue(t *testing.T, section, name string) string {
	t.Helper()
	for _, e := range readLock(t) {
		if e.section == section && e.name == name {
			return e.value
		}
	}
	t.Fatalf("tools.lock has no %s entry %q", section, name)
	return ""
}

func TestToolsLockPinsEveryExternalTool(t *testing.T) {
	want := map[string]string{
		"deadcode": "go", "govulncheck": "go", "gremlins": "go",
		"golangci-lint": "release", "ast-grep": "release",
	}
	got := map[string]string{}
	for _, e := range readLock(t) {
		if e.section == "go" || e.section == "release" {
			got[e.name] = e.section
		}
	}
	for name, section := range want {
		if got[name] != section {
			t.Errorf("%s: want pinned in [%s], got [%s]", name, section, got[name])
		}
	}
	for name := range got {
		if _, ok := want[name]; !ok {
			t.Errorf("%s is pinned but no test knows what uses it", name)
		}
	}
}

func TestToolsLockVersionsArePinnedExactly(t *testing.T) {
	for _, e := range readLock(t) {
		switch e.section {
		case "go":
			_, version, ok := strings.Cut(e.value, "@v")
			if !ok || strings.ContainsAny(version, "*^~<>") || version == "latest" {
				t.Errorf("%s: %q is not package@vX.Y.Z", e.name, e.value)
			}
		case "release":
			if strings.Trim(e.value, "0123456789.") != "" {
				t.Errorf("%s: %q is not an exact version", e.name, e.value)
			}
		case "checksums":
			if len(e.value) != 64 || strings.Trim(e.value, "0123456789abcdef") != "" {
				t.Errorf("%s: %q is not a sha256", e.name, e.value)
			}
		default:
			t.Errorf("tools.lock: unknown section [%s]", e.section)
		}
	}
	if lockValue(t, "checksums", "ast-grep") == "" {
		t.Error("ast-grep publishes no checksum file, so tools.lock must carry its sha256")
	}
}

type installRun struct {
	out  string
	code int
}

// runInstall runs scripts/install-tools.sh with PATH holding only the stub executables given
// (name -> version line printed by --version) and the system utilities.
func runInstall(t *testing.T, stubs map[string]string, args ...string) installRun {
	t.Helper()
	script, err := filepath.Abs("../../scripts/install-tools.sh")
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	for name, line := range stubs {
		writeStub(t, bin, name, "#!/bin/sh\necho '"+line+"'\n")
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(t.Context(), bash, append([]string{script}, args...)...)
	cmd.Env = append(os.Environ(), "PATH="+bin+":/usr/bin:/bin")
	out, runErr := cmd.CombinedOutput()
	code := 0
	var exitErr *exec.ExitError
	if runErr != nil {
		if !errors.As(runErr, &exitErr) {
			t.Fatal(runErr)
		}
		code = exitErr.ExitCode()
	}
	return installRun{string(out), code}
}

func pinnedStubs(t *testing.T) map[string]string {
	t.Helper()
	return map[string]string{
		"golangci-lint": "golangci-lint has version " + lockValue(t, "release", "golangci-lint") + " built with go1 from abc on today",
		"ast-grep":      "ast-grep " + lockValue(t, "release", "ast-grep"),
	}
}

func TestInstallToolsRefusesAnUnpinnedTool(t *testing.T) {
	r := runInstall(t, nil, "--check", "not-a-pinned-tool")
	if r.code != 2 || !strings.Contains(r.out, "not-a-pinned-tool is not pinned in tools.lock") {
		t.Fatalf("want exit 2 naming the tool, got %d: %s", r.code, r.out)
	}
}

func TestInstallToolsCheckReportsAMissingTool(t *testing.T) {
	r := runInstall(t, nil, "--check", "ast-grep")
	if r.code != 1 || !strings.Contains(r.out, "MISSING   ast-grep "+lockValue(t, "release", "ast-grep")) {
		t.Fatalf("want exit 1 and MISSING, got %d: %s", r.code, r.out)
	}
}

func TestInstallToolsCheckAcceptsThePinnedVersions(t *testing.T) {
	r := runInstall(t, pinnedStubs(t), "--check", "golangci-lint", "ast-grep")
	if r.code != 0 || strings.Count(r.out, "ok        ") != 2 {
		t.Fatalf("want exit 0 and two ok lines, got %d: %s", r.code, r.out)
	}
}

func TestInstallToolsCheckRejectsAnotherVersion(t *testing.T) {
	stubs := pinnedStubs(t)
	stubs["ast-grep"] = "ast-grep 0.0.1"
	r := runInstall(t, stubs, "--check", "golangci-lint", "ast-grep")
	if r.code != 1 || !strings.Contains(r.out, "installed: 0.0.1") || !strings.Contains(r.out, "ok        golangci-lint") {
		t.Fatalf("want exit 1 naming the installed version, got %d: %s", r.code, r.out)
	}
}

func TestInstallToolsCheckWithoutSelectionCoversEveryPinnedTool(t *testing.T) {
	r := runInstall(t, nil, "--check")
	if r.code != 1 {
		t.Fatalf("want exit 1, got %d: %s", r.code, r.out)
	}
	for _, e := range readLock(t) {
		if e.section == "checksums" {
			continue
		}
		if !strings.Contains(r.out, "MISSING   "+e.name+" ") {
			t.Errorf("%s is pinned but --check does not report it: %s", e.name, r.out)
		}
	}
}
