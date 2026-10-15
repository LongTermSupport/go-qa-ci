package toolstore_test

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/faketool"
	"github.com/LongTermSupport/go-qa-ci/internal/toolstore"
)

const linuxAMD64 = "linux/amd64"

func sum(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func tarGz(t *testing.T, member string, body []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: member, Mode: 0o755, Size: int64(len(body))}); err != nil {
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

func zipOf(t *testing.T, member string, body []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create(member)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = w.Write(body); err != nil {
		t.Fatal(err)
	}
	if err = zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// fixture is a store over a temporary directory whose downloader serves archives from a map and
// whose go install writes a marker program; calls are counted.
type fixture struct {
	store     *toolstore.Store
	archives  map[string][]byte
	downloads []string
	goInstall []string
	log       bytes.Buffer
}

func lockText(sumGolangci, sumAstGrep string) string {
	return "[go]\ndeadcode = \"golang.org/x/tools/cmd/deadcode@v0.52.0\"\n" +
		"[release]\ngolangci-lint = \"2.14.0\"\nast-grep = \"0.50.0\"\n" +
		"[checksums]\n" + checksumLine("golangci-lint", sumGolangci) + checksumLine("ast-grep", sumAstGrep)
}

func checksumLine(name, value string) string {
	if value == "" {
		return ""
	}
	return name + " = \"" + value + "\"\n"
}

const (
	golangciURL = "https://github.com/golangci/golangci-lint/releases/download/v2.14.0/golangci-lint-2.14.0-linux-amd64.tar.gz"
	astGrepURL  = "https://github.com/ast-grep/ast-grep/releases/download/0.50.0/app-x86_64-unknown-linux-gnu.zip"
)

func newFixture(t *testing.T, golangciSum, astGrepSum string) *fixture {
	t.Helper()
	lock, err := toolstore.ParseLock(lockText(golangciSum, astGrepSum))
	if err != nil {
		t.Fatal(err)
	}
	f := &fixture{archives: map[string][]byte{
		golangciURL: tarGz(t, "golangci-lint-2.14.0-linux-amd64/golangci-lint", []byte("golangci-binary")),
		astGrepURL:  zipOf(t, "ast-grep", []byte("ast-grep-binary")),
	}}
	f.store = &toolstore.Store{
		Dir:      t.TempDir(),
		Lock:     lock,
		Platform: linuxAMD64,
		Log:      &f.log,
		Download: func(_ context.Context, url string, w io.Writer) error {
			f.downloads = append(f.downloads, url)
			data, ok := f.archives[url]
			if !ok {
				return errors.New("no such url " + url)
			}
			if _, err := w.Write(data); err != nil {
				return fmt.Errorf("serving %s: %w", url, err)
			}
			return nil
		},
		GoInstall: func(_ context.Context, pkg, gobin string) error {
			f.goInstall = append(f.goInstall, pkg+" -> "+gobin)
			return os.WriteFile(filepath.Join(gobin, "deadcode"), []byte("deadcode-binary"), 0o600)
		},
	}
	return f
}

func goodFixture(t *testing.T) *fixture {
	t.Helper()
	probe := newFixture(t, "", "")
	return newFixture(t, sum(probe.archives[golangciURL]), sum(probe.archives[astGrepURL]))
}

func entries(t *testing.T, dir string) []string {
	t.Helper()
	var names []string
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			names = append(names, p)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return names
}

func TestInstallPlacesEveryToolUnderItsVersion(t *testing.T) {
	f := goodFixture(t)
	if err := f.store.Install(t.Context()); err != nil {
		t.Fatal(err)
	}
	for name, version := range map[string]string{"golangci-lint": "2.14.0", "ast-grep": "0.50.0", "deadcode": "v0.52.0"} {
		path, ok := f.store.Resolve(name)
		want := filepath.Join(f.store.Dir, name, version, name)
		if !ok || path != want {
			t.Errorf("%s: resolved %q (%v), want %q", name, path, ok, want)
		}
		if info, err := os.Stat(want); err != nil || (name != "deadcode" && info.Mode().Perm()&0o100 == 0) {
			t.Errorf("%s: not an executable file: %v", name, err)
		}
	}
	if got := f.goInstall; len(got) != 1 || !strings.HasPrefix(got[0], "golang.org/x/tools/cmd/deadcode@v0.52.0 -> ") {
		t.Errorf("go install calls: %v", got)
	}
}

func TestInstallRefusesAChecksumMismatchAndLeavesNothing(t *testing.T) {
	f := newFixture(t, sum([]byte("another archive")), "")
	err := f.store.Install(t.Context(), "golangci-lint")
	if err == nil || !strings.Contains(err.Error(), "sha256") || !strings.Contains(err.Error(), "golangci-lint") {
		t.Fatalf("want a sha256 mismatch naming the tool, got %v", err)
	}
	if left := entries(t, f.store.Dir); len(left) != 0 {
		t.Errorf("a refused install left files behind: %v", left)
	}
	if _, ok := f.store.Resolve("golangci-lint"); ok {
		t.Error("a refused install resolves")
	}
}

func TestInstallRefusesAReleaseToolWithNoPinnedChecksumBeforeDownloading(t *testing.T) {
	f := newFixture(t, "", "")
	err := f.store.Install(t.Context(), "ast-grep")
	if err == nil || !strings.Contains(err.Error(), "[checksums]") {
		t.Fatalf("want a refusal naming [checksums], got %v", err)
	}
	if len(f.downloads) != 0 {
		t.Errorf("downloaded before refusing: %v", f.downloads)
	}
}

func TestInstallRefusesAnUnpinnedName(t *testing.T) {
	f := goodFixture(t)
	if err := f.store.Install(t.Context(), "nope"); err == nil || !strings.Contains(err.Error(), "nope is not pinned") {
		t.Fatalf("got %v", err)
	}
}

func TestSecondInstallDoesNothing(t *testing.T) {
	f := goodFixture(t)
	if err := f.store.Install(t.Context()); err != nil {
		t.Fatal(err)
	}
	downloads, installs := len(f.downloads), len(f.goInstall)
	if err := f.store.Install(t.Context()); err != nil {
		t.Fatal(err)
	}
	if len(f.downloads) != downloads || len(f.goInstall) != installs {
		t.Errorf("second install fetched again: downloads %v, go install %v", f.downloads, f.goInstall)
	}
}

func TestAFailedDownloadLeavesNothingInPlace(t *testing.T) {
	f := goodFixture(t)
	delete(f.archives, astGrepURL)
	if err := f.store.Install(t.Context(), "ast-grep"); err == nil || !strings.Contains(err.Error(), "ast-grep") {
		t.Fatalf("want an error naming the tool, got %v", err)
	}
	if left := entries(t, f.store.Dir); len(left) != 0 {
		t.Errorf("left files behind: %v", left)
	}
}

func TestAnArchiveWithoutTheBinaryLeavesNothingInPlace(t *testing.T) {
	probe := goodFixture(t)
	archive := tarGz(t, "golangci-lint-2.14.0-linux-amd64/README", []byte("x"))
	f := newFixture(t, sum(archive), sum(probe.archives[astGrepURL]))
	f.archives[golangciURL] = archive
	err := f.store.Install(t.Context(), "golangci-lint")
	if err == nil || !strings.Contains(err.Error(), "golangci-lint-2.14.0-linux-amd64/golangci-lint") {
		t.Fatalf("want the missing member named, got %v", err)
	}
	if left := entries(t, f.store.Dir); len(left) != 0 {
		t.Errorf("left files behind: %v", left)
	}
}

func TestAFailedGoInstallLeavesNothingInPlace(t *testing.T) {
	f := goodFixture(t)
	f.store.GoInstall = func(context.Context, string, string) error { return errors.New("build failed") }
	if err := f.store.Install(t.Context(), "deadcode"); err == nil || !strings.Contains(err.Error(), "build failed") {
		t.Fatalf("got %v", err)
	}
	if left := entries(t, f.store.Dir); len(left) != 0 {
		t.Errorf("left files behind: %v", left)
	}
}

func TestAGoInstallThatBuiltNothingIsAnError(t *testing.T) {
	f := goodFixture(t)
	f.store.GoInstall = func(context.Context, string, string) error { return nil }
	if err := f.store.Install(t.Context(), "deadcode"); err == nil {
		t.Fatal("want an error: nothing was built")
	}
}

func TestReleaseToolsNeedLinuxAMD64(t *testing.T) {
	f := goodFixture(t)
	f.store.Platform = "darwin/arm64"
	err := f.store.Install(t.Context(), "golangci-lint")
	if err == nil || !strings.Contains(err.Error(), "darwin/arm64") || !strings.Contains(err.Error(), linuxAMD64) {
		t.Fatalf("want the platform named, got %v", err)
	}
	if len(f.downloads) != 0 {
		t.Errorf("downloaded on an unsupported platform: %v", f.downloads)
	}
}

func TestCheckReportsWhatIsMissing(t *testing.T) {
	f := goodFixture(t)
	if got := f.store.Missing(); strings.Join(got, ",") != "ast-grep,deadcode,golangci-lint" {
		t.Errorf("missing before install: %v", got)
	}
	if err := f.store.Install(t.Context(), "ast-grep"); err != nil {
		t.Fatal(err)
	}
	if got := f.store.Missing(); strings.Join(got, ",") != "deadcode,golangci-lint" {
		t.Errorf("missing after one install: %v", got)
	}
}

func TestEnsureInstallsWhatIsMissingAndPutsThePinnedCopyFirstOnPath(t *testing.T) {
	f := goodFixture(t)
	faketool.Install(t, "ast-grep", faketool.Spec{})
	if err := f.store.Ensure(t.Context(), "ast-grep"); err != nil {
		t.Fatal(err)
	}
	found, err := exec.LookPath("ast-grep")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(f.store.Dir, "ast-grep", "0.50.0", "ast-grep"); found != want {
		t.Errorf("a PATH copy answers for the pinned tool: %s, want %s", found, want)
	}
	if len(f.downloads) != 1 {
		t.Errorf("downloads: %v", f.downloads)
	}
}

func TestEnsureFetchesNothingForAnInstalledTool(t *testing.T) {
	f := goodFixture(t)
	t.Setenv("PATH", os.Getenv("PATH"))
	if err := f.store.Install(t.Context(), "ast-grep"); err != nil {
		t.Fatal(err)
	}
	f.downloads = nil
	if err := f.store.Ensure(t.Context(), "ast-grep"); err != nil {
		t.Fatal(err)
	}
	if len(f.downloads) != 0 {
		t.Errorf("fetched an installed tool: %v", f.downloads)
	}
}

func TestDefaultDirPrefersTheEnvironmentVariable(t *testing.T) {
	t.Setenv(toolstore.DirEnv, "/somewhere/tools")
	dir, err := toolstore.DefaultDir()
	if err != nil || dir != "/somewhere/tools" {
		t.Fatalf("got %q, %v", dir, err)
	}
	t.Setenv(toolstore.DirEnv, "")
	t.Setenv("XDG_CACHE_HOME", "/cache")
	dir, err = toolstore.DefaultDir()
	if err != nil || dir != "/cache/go-qa/tools" {
		t.Fatalf("got %q, %v", dir, err)
	}
}

func TestDefaultDirRefusesARelativeOverride(t *testing.T) {
	t.Setenv(toolstore.DirEnv, "relative/dir")
	if _, err := toolstore.DefaultDir(); err == nil {
		t.Fatal("want an error for a relative directory")
	}
}
