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

func TestToolsLockPinsEveryExternalTool(t *testing.T) {
	want := map[string]string{
		"deadcode": "go", "govulncheck": "go", "gremlins": "go",
		"golangci-lint": "release", "ast-grep": "release", "gitleaks": "release", "actionlint": "release",
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
}

func TestEveryReleaseToolPinsTheSha256OfItsArchive(t *testing.T) {
	pinned := map[string]bool{}
	for _, e := range readLock(t) {
		if e.section == "checksums" {
			pinned[e.name] = true
		}
	}
	for _, e := range readLock(t) {
		if e.section == "release" && !pinned[e.name] {
			t.Errorf("%s is a [release] tool with no sha256 in [checksums]: trusting the checksum file of the same release lets a replaced asset vouch for itself", e.name)
		}
	}
}

// The script is a thin call of go-qa install-tools: the behaviour (checksums, atomic placement,
// idempotence) is proved by the internal/toolstore tests, not by a second implementation here.
func TestInstallToolsScriptCallsGoQaInstallTools(t *testing.T) {
	script, err := filepath.Abs("../../scripts/install-tools.sh")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(t.Context(), "bash", script, "--check", "ast-grep")
	cmd.Env = append(os.Environ(), "GOQA_TOOLS_DIR="+t.TempDir())
	out, runErr := cmd.CombinedOutput()
	var exitErr *exec.ExitError
	if !errors.As(runErr, &exitErr) || exitErr.ExitCode() != 1 || !strings.Contains(string(out), "ast-grep is not installed") {
		t.Fatalf("want exit 1 naming ast-grep as not installed, got %v: %s", runErr, out)
	}
}

func TestInstallToolsScriptRefusesAnUnpinnedTool(t *testing.T) {
	script, err := filepath.Abs("../../scripts/install-tools.sh")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(t.Context(), "bash", script, "--check", "not-a-pinned-tool")
	cmd.Env = append(os.Environ(), "GOQA_TOOLS_DIR="+t.TempDir())
	out, runErr := cmd.CombinedOutput()
	var exitErr *exec.ExitError
	if !errors.As(runErr, &exitErr) || exitErr.ExitCode() != 2 || !strings.Contains(string(out), "not-a-pinned-tool is not pinned in tools.lock") {
		t.Fatalf("want exit 2 naming the tool, got %v: %s", runErr, out)
	}
}
