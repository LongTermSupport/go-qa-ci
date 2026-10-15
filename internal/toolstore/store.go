package toolstore

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/LongTermSupport/go-qa-ci/internal/runners"
)

// DirEnv names the one environment variable that moves the tool directory.
const DirEnv = "GOQA_TOOLS_DIR"

const (
	downloadTimeout = 15 * time.Minute
	dirMode         = 0o750
	// DefaultMaxArchiveBytes bounds a release download: the pinned archives are tens of megabytes,
	// so 256 MiB leaves room to grow while a wrong or hostile URL cannot fill memory.
	DefaultMaxArchiveBytes = 256 << 20
)

// Store is the directory the pinned tools are installed into. Dir/<name>/<version>/<name> is a
// release tool; a Go tool's version directory also names the Go that built it
// (<version>+<goversion>), so a toolchain change reinstalls it. The Download, GoInstall and
// GoVersion fields are the seams the tests replace.
type Store struct {
	// Dir holds the tools; it must be absolute.
	Dir string
	// Lock is the pin set.
	Lock Lock
	// Platform is "GOOS/GOARCH"; release archives exist for ReleasePlatform only.
	Platform string
	// Log receives one line for each tool installed.
	Log io.Writer
	// Download writes the body of the https URL to w.
	Download func(ctx context.Context, url string, w io.Writer) error
	// GoInstall runs `go install pkg` (package@version) with GOBIN set to gobin.
	GoInstall func(ctx context.Context, pkg, gobin string) error
	// GoVersion is the version of the go command that builds the Go tools.
	GoVersion func() (string, error)
	// MaxArchiveBytes caps a release download; zero means DefaultMaxArchiveBytes.
	MaxArchiveBytes int64
}

// New is a Store that uses the network and the go command.
func New(lock Lock, dir string, log io.Writer) *Store {
	return &Store{
		Dir:       dir,
		Lock:      lock,
		Platform:  runtime.GOOS + "/" + runtime.GOARCH,
		Log:       log,
		Download:  httpDownload,
		GoInstall: goInstall,
		GoVersion: goVersion(),
	}
}

// DefaultDir is the tool directory: $GOQA_TOOLS_DIR, else <user cache dir>/go-qa/tools.
func DefaultDir() (string, error) {
	if dir := os.Getenv(DirEnv); dir != "" {
		if !filepath.IsAbs(dir) {
			return "", fmt.Errorf("%s=%q must be an absolute path", DirEnv, dir)
		}
		return dir, nil
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("no cache directory for the pinned tools (set %s): %w", DirEnv, err)
	}
	return filepath.Join(cache, "go-qa", "tools"), nil
}

// dir is where the tool is installed; a Go tool's directory depends on the Go that builds it.
func (s *Store) dir(tool Tool) (string, error) {
	key := tool.Version
	if tool.Kind == KindGo {
		goVersion, err := s.GoVersion()
		if err != nil {
			return "", fmt.Errorf("the go version that builds %s: %w", tool.Name, err)
		}
		key += "+" + goVersion
	}
	return filepath.Join(s.Dir, tool.Name, key), nil
}

// Resolve is the path of the installed pinned copy of name.
func (s *Store) Resolve(name string) (string, bool) {
	tool, err := s.Lock.Tool(name)
	if err != nil {
		return "", false
	}
	dir, err := s.dir(tool)
	if err != nil {
		return "", false
	}
	path := filepath.Join(dir, name)
	info, statErr := os.Stat(path)
	if statErr != nil || !info.Mode().IsRegular() {
		return "", false
	}
	return path, true
}

// Missing lists the pinned tools that are not installed, sorted.
func (s *Store) Missing() []string {
	var missing []string
	for _, name := range s.Lock.Names() {
		if _, ok := s.Resolve(name); !ok {
			missing = append(missing, name)
		}
	}
	return missing
}

// Install installs the named tools (every pinned tool when none are named) that are not installed.
func (s *Store) Install(ctx context.Context, names ...string) error {
	if len(names) == 0 {
		names = s.Lock.Names()
	}
	tools := make([]Tool, 0, len(names))
	for _, name := range names {
		tool, err := s.Lock.Tool(name)
		if err != nil {
			return err
		}
		tools = append(tools, tool)
	}
	for _, tool := range tools {
		if _, ok := s.Resolve(tool.Name); ok {
			continue
		}
		if err := s.install(ctx, tool); err != nil {
			return fmt.Errorf("installing %s %s: %w", tool.Name, tool.Version, err)
		}
		if _, err := fmt.Fprintf(s.Log, "installed %s %s\n", tool.Name, tool.Version); err != nil {
			return fmt.Errorf("writing the install log: %w", err)
		}
	}
	return nil
}

// Ensure installs what is missing, then puts the directory of every named pinned tool first on
// PATH, so the pinned copy answers for the name and a copy elsewhere on PATH never does.
func (s *Store) Ensure(ctx context.Context, names ...string) error {
	if err := s.Install(ctx, names...); err != nil {
		return err
	}
	if len(names) == 0 {
		names = s.Lock.Names()
	}
	dirs := make([]string, 0, len(names)+1)
	for _, name := range names {
		path, ok := s.Resolve(name)
		if !ok {
			return fmt.Errorf("%s is not installed after installing it", name)
		}
		dirs = append(dirs, filepath.Dir(path))
	}
	dirs = append(dirs, os.Getenv("PATH"))
	if err := os.Setenv("PATH", strings.Join(dirs, string(os.PathListSeparator))); err != nil {
		return fmt.Errorf("putting the pinned tools on PATH: %w", err)
	}
	return nil
}

// install builds the tool in a scratch directory beside its final place and renames it in.
func (s *Store) install(ctx context.Context, tool Tool) (err error) {
	if err = s.checkInstallable(tool); err != nil {
		return err
	}
	final, err := s.dir(tool)
	if err != nil {
		return err
	}
	parent := filepath.Join(s.Dir, tool.Name)
	if err = os.MkdirAll(parent, dirMode); err != nil {
		return fmt.Errorf("creating %s: %w", parent, err)
	}
	scratch, err := os.MkdirTemp(parent, ".install-")
	if err != nil {
		return fmt.Errorf("creating a scratch directory: %w", err)
	}
	defer func() {
		if removeErr := os.RemoveAll(scratch); removeErr != nil && err == nil {
			err = fmt.Errorf("removing %s: %w", scratch, removeErr)
		}
		if err != nil {
			err = errors.Join(err, removeIfEmpty(parent))
		}
	}()
	stage := filepath.Join(scratch, "stage")
	if err = os.Mkdir(stage, dirMode); err != nil {
		return fmt.Errorf("creating %s: %w", stage, err)
	}
	if tool.Kind == KindGo {
		err = s.buildGo(ctx, tool, stage)
	} else {
		err = s.fetchRelease(ctx, tool, stage)
	}
	if err != nil {
		return err
	}
	if _, statErr := os.Stat(filepath.Join(stage, tool.Name)); statErr != nil {
		return fmt.Errorf("the install produced no %s: %w", tool.Name, statErr)
	}
	if err = os.Rename(stage, final); err != nil {
		if _, ok := s.Resolve(tool.Name); ok {
			return nil // another go-qa installed the same version first
		}
		return fmt.Errorf("placing the tool: %w", err)
	}
	return nil
}

// checkInstallable refuses, before anything is fetched, what cannot be installed verified.
func (s *Store) checkInstallable(tool Tool) error {
	if tool.Kind != KindRelease {
		return nil
	}
	if s.Platform != ReleasePlatform {
		return fmt.Errorf("release archives are pinned for %s only, not %s", ReleasePlatform, s.Platform)
	}
	if _, known := releases[tool.Name]; !known {
		return fmt.Errorf("no archive layout is known for %s", tool.Name)
	}
	if tool.SHA256 == "" {
		return fmt.Errorf("%s is a [release] tool, so tools.lock [checksums] must pin its sha256", tool.Name)
	}
	return nil
}

func (s *Store) buildGo(ctx context.Context, tool Tool, stage string) error {
	if err := s.GoInstall(ctx, tool.Package+"@"+tool.Version, stage); err != nil {
		return fmt.Errorf("go install: %w", err)
	}
	return nil
}

// fetchRelease downloads the archive, verifies it against the pinned sha256 and only then
// extracts the binary.
func (s *Store) fetchRelease(ctx context.Context, tool Tool, stage string) error {
	spec := releases[tool.Name]
	limit := s.MaxArchiveBytes
	if limit == 0 {
		limit = DefaultMaxArchiveBytes
	}
	var archive bytes.Buffer
	hash := sha256.New()
	capped := &cappedWriter{w: io.MultiWriter(&archive, hash), left: limit}
	if err := s.Download(ctx, spec.url(tool.Version), capped); err != nil {
		if errors.Is(err, errArchiveTooLarge) {
			return fmt.Errorf("the archive at %s exceeds %d bytes", spec.url(tool.Version), limit)
		}
		return fmt.Errorf("downloading %s: %w", spec.url(tool.Version), err)
	}
	if got := hex.EncodeToString(hash.Sum(nil)); got != tool.SHA256 {
		return fmt.Errorf("the archive has sha256 %s, tools.lock pins %s", got, tool.SHA256)
	}
	return extract(archive.Bytes(), spec.zipped, spec.member(tool.Version), filepath.Join(stage, tool.Name))
}

func httpDownload(ctx context.Context, url string, w io.Writer) error {
	if !strings.HasPrefix(url, "https://") {
		return fmt.Errorf("refusing the non-https URL %s", url)
	}
	ctx, cancel := context.WithTimeout(ctx, downloadTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return fmt.Errorf("building the request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("fetching: %w", err)
	}
	defer func() { err = errors.Join(err, resp.Body.Close()) }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server answered %s", resp.Status)
	}
	if _, err = io.Copy(w, resp.Body); err != nil {
		return fmt.Errorf("reading the body: %w", err)
	}
	return nil
}

func goInstall(ctx context.Context, pkg, gobin string) error {
	goBinary, err := exec.LookPath("go")
	if err != nil {
		return fmt.Errorf("the go command is needed to build %s: %w", pkg, err)
	}
	out, err := runners.CaptureEnv(ctx, "", goInstallEnv(gobin), goBinary, "install", pkg)
	if err != nil {
		return fmt.Errorf("go install: %w", err)
	}
	if out.Code != 0 {
		return fmt.Errorf("go install exited %d: %s", out.Code, strings.TrimSpace(out.Combined()))
	}
	return nil
}

// Names are the pinned tools, sorted.
func (s *Store) Names() []string { return s.Lock.Names() }

// goInstallEnv makes `go install pkg@version` independent of the consumer's module: -mod=vendor
// cannot query a module, and a go.work would add its own requirements. GOTOOLCHAIN and GOPROXY
// stay the consumer's.
func goInstallEnv(gobin string) []string {
	return []string{"GOBIN=" + gobin, "GOFLAGS=-mod=mod", "GOWORK=off"}
}

var errArchiveTooLarge = errors.New("archive too large")

// cappedWriter fails the write that would take the total past the cap, so an oversized download
// stops before it is held in memory.
type cappedWriter struct {
	w    io.Writer
	left int64
}

func (c *cappedWriter) Write(p []byte) (int, error) {
	if int64(len(p)) > c.left {
		return 0, errArchiveTooLarge
	}
	c.left -= int64(len(p))
	n, err := c.w.Write(p)
	if err != nil {
		return n, fmt.Errorf("buffering the archive: %w", err)
	}
	return n, nil
}

// removeIfEmpty removes dir after a failed install when nothing else lives in it; a directory
// another install is using is left alone.
func removeIfEmpty(dir string) error {
	err := os.Remove(dir)
	if err == nil || errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ENOTEMPTY) {
		return nil
	}
	return fmt.Errorf("removing %s: %w", dir, err)
}

// goVersion asks the go command that builds the Go tools for its version, once.
func goVersion() func() (string, error) {
	var once sync.Once
	var version string
	var err error
	return func() (string, error) {
		once.Do(func() { version, err = queryGoVersion() })
		return version, err
	}
}

func queryGoVersion() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	goBinary, err := exec.LookPath("go")
	if err != nil {
		return "", fmt.Errorf("the go command is needed to build the Go tools: %w", err)
	}
	out, err := runners.Capture(ctx, "", goBinary, "env", "GOVERSION")
	if err != nil {
		return "", fmt.Errorf("go env GOVERSION: %w", err)
	}
	version := strings.TrimSpace(out.Stdout)
	if out.Code != 0 || version == "" {
		return "", fmt.Errorf("go env GOVERSION exited %d: %s", out.Code, strings.TrimSpace(out.Combined()))
	}
	return version, nil
}
