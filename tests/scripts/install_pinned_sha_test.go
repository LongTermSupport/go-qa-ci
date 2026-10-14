package scripts_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fixtureVersion is the version the test locks pin; the stub curl serves a fixture archive for it.
const fixtureVersion = "9.9.9"

// archiveTool describes how install-tools.sh unpacks a release tool: the member of the archive
// holding the binary, and what that binary prints when asked its version.
type archiveTool struct {
	name    string
	member  string
	version string
}

var archiveTools = []archiveTool{
	{"golangci-lint", "golangci-lint-" + fixtureVersion + "-linux-amd64/golangci-lint", "golangci-lint has version " + fixtureVersion + " built with go1 from abc on today"},
	{"gitleaks", "gitleaks", fixtureVersion},
	{"actionlint", "actionlint", fixtureVersion},
}

// fixtureArchive is a gzip tarball holding one executable that prints the tool's version line.
func fixtureArchive(t *testing.T, tool archiveTool) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	body := []byte("#!/bin/sh\necho '" + tool.version + "'\n")
	if err := tw.WriteHeader(&tar.Header{Name: tool.member, Mode: 0o755, Size: int64(len(body))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

const curlStub = `#!/bin/sh
out=""
url=""
while [ $# -gt 0 ]; do
	case "$1" in
		-o) out="$2"; shift ;;
		--proto) shift ;;
		-*) ;;
		*) url="$1" ;;
	esac
	shift
done
echo "$url" >> "$CURL_LOG"
cp "$FIXTURE" "$out"
`

type pinnedInstall struct {
	out     string
	code    int
	urls    []string
	binPath string
}

// runPinnedInstall runs install-tools.sh for one release tool against a lock pinning the given
// sha256 (none when sum is empty), with a stub curl serving archive for every download.
func runPinnedInstall(t *testing.T, tool archiveTool, archive []byte, sum string) pinnedInstall {
	t.Helper()
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		t.Skip("release archives are pinned for linux/amd64 only")
	}
	script, err := filepath.Abs("../../scripts/install-tools.sh")
	if err != nil {
		t.Fatal(err)
	}
	work := t.TempDir()
	gobin := filepath.Join(work, "gobin")
	stubs := filepath.Join(work, "stubs")
	for _, dir := range []string{gobin, stubs} {
		if mkErr := os.MkdirAll(dir, 0o750); mkErr != nil {
			t.Fatal(mkErr)
		}
	}
	writeStub(t, stubs, "curl", curlStub)
	fixture := filepath.Join(work, "fixture.tar.gz")
	writeFile(t, fixture, string(archive))
	curlLog := filepath.Join(work, "curl.log")
	writeFile(t, curlLog, "")
	lock := "[release]\n" + tool.name + " = \"" + fixtureVersion + "\"\n"
	if sum != "" {
		lock += "\n[checksums]\n" + tool.name + " = \"" + sum + "\"\n"
	}
	lockPath := filepath.Join(work, "tools.lock")
	writeFile(t, lockPath, lock)
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(t.Context(), bash, script, tool.name)
	cmd.Env = append(os.Environ(),
		"PATH="+gobin+":"+stubs+":/usr/bin:/bin", "GOBIN="+gobin, "GQACI_TOOLS_LOCK="+lockPath,
		"FIXTURE="+fixture, "CURL_LOG="+curlLog)
	out, runErr := cmd.CombinedOutput()
	code := 0
	var exitErr *exec.ExitError
	if runErr != nil {
		if !errors.As(runErr, &exitErr) {
			t.Fatal(runErr)
		}
		code = exitErr.ExitCode()
	}
	scratch, err := os.OpenRoot(work)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := scratch.Close(); closeErr != nil {
			t.Error(closeErr)
		}
	}()
	logged, err := scratch.ReadFile("curl.log")
	if err != nil {
		t.Fatal(err)
	}
	return pinnedInstall{
		out: string(out), code: code, binPath: filepath.Join(gobin, tool.name),
		urls: strings.Fields(string(logged)),
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

func TestInstallToolsInstallsAnArchiveMatchingItsPinnedSha256(t *testing.T) {
	for _, tool := range archiveTools {
		t.Run(tool.name, func(t *testing.T) {
			archive := fixtureArchive(t, tool)
			r := runPinnedInstall(t, tool, archive, sha256Hex(archive))
			if r.code != 0 || !strings.Contains(r.out, "installed "+tool.name+" "+fixtureVersion) {
				t.Fatalf("want a clean install, got %d: %s", r.code, r.out)
			}
			if _, err := os.Stat(r.binPath); err != nil {
				t.Errorf("the binary was not installed: %v", err)
			}
			for _, url := range r.urls {
				if strings.Contains(url, "checksum") {
					t.Errorf("fetched %s: verification must use the pinned sha256, not a checksum file from the same release", url)
				}
			}
			if len(r.urls) != 1 {
				t.Errorf("want one download (the archive), got %v", r.urls)
			}
		})
	}
}

func TestInstallToolsRefusesAnArchiveThatDiffersFromItsPinnedSha256(t *testing.T) {
	for _, tool := range archiveTools {
		t.Run(tool.name, func(t *testing.T) {
			archive := fixtureArchive(t, tool)
			r := runPinnedInstall(t, tool, archive, sha256Hex([]byte("another archive")))
			if r.code == 0 || !strings.Contains(r.out, "FAILED") {
				t.Fatalf("want a checksum failure, got %d: %s", r.code, r.out)
			}
			if _, err := os.Stat(r.binPath); err == nil {
				t.Error("a binary that failed verification was installed")
			}
		})
	}
}

func TestInstallToolsRefusesAReleaseToolWithNoPinnedSha256(t *testing.T) {
	for _, tool := range archiveTools {
		t.Run(tool.name, func(t *testing.T) {
			r := runPinnedInstall(t, tool, fixtureArchive(t, tool), "")
			if r.code != 2 || !strings.Contains(r.out, "[checksums] must pin its sha256") || len(r.urls) != 0 {
				t.Fatalf("want exit 2 before any download, got %d (%v): %s", r.code, r.urls, r.out)
			}
		})
	}
}
