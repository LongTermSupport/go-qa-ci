// Package suppression finds every route by which a finding can be silenced without an entry in
// the project record (TOOLING-SPEC 4.3) and turns each into a finding unless the record licenses it.
//
// The routes are: the comment directives of golangci-lint, staticcheck, gosec and ast-grep in Go
// source; a hand-written generated-code header, which makes golangci-lint and staticcheck skip
// the file; staticcheck.conf; and the exclusion and disabling settings of golangci-lint
// configuration. A file that cannot be parsed is reported, because what it silences is unknown.
// Everything is read from syntax, never from type information, so no build or tool run is needed.
package suppression

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"slices"
	"strings"

	"github.com/LongTermSupport/go-qa-ci/internal/glob"
	"github.com/LongTermSupport/go-qa-ci/internal/root"
)

// Route says how a suppression is expressed.
type Route string

// The suppression routes.
const (
	// RouteNolint is a golangci-lint nolint directive.
	RouteNolint Route = "nolint directive"
	// RouteLintIgnore is a staticcheck line or file ignore directive.
	RouteLintIgnore Route = "staticcheck ignore directive"
	// RouteGosec is a gosec nosec marker.
	RouteGosec Route = "gosec nosec marker"
	// RouteAstGrep is an ast-grep ignore comment.
	RouteAstGrep Route = "ast-grep ignore comment"
	// RouteGeneratedHeader is a generated-code header, which exempts the file from linting.
	RouteGeneratedHeader Route = "generated-code header"
	// RouteStaticcheckConf is a setting in staticcheck.conf that silences checks.
	RouteStaticcheckConf Route = "staticcheck.conf setting"
	// RouteGolangciConfig is an exclusion or disabling setting in golangci-lint configuration.
	RouteGolangciConfig Route = "golangci-lint configuration setting"
)

// Suppression is one thing that silences a defence.
type Suppression struct {
	// Rule is the identifier being silenced, as the defence prints it; empty when the directive
	// names none, which no record entry can license.
	Rule string
	// Path is the file, relative to the root and slash-separated.
	Path string
	// Line is 1-based; 0 when the format has no usable line.
	Line int
	// Route is how the suppression is expressed.
	Route Route
	// Bare says why the suppression is not acceptable even when recorded: it names no rule, or
	// gives no reason. Empty when it is acceptable.
	Bare string
}

// String renders the suppression compactly: rule@path:line, with " bare" when Bare is set.
func (s Suppression) String() string {
	rule := s.Rule
	if rule == "" {
		rule = "(none)"
	}
	text := fmt.Sprintf("%s@%s:%d", rule, s.Path, s.Line)
	if s.Bare != "" {
		text += " bare"
	}
	return text
}

// Unparseable is a file whose suppressions cannot be determined.
type Unparseable struct {
	// Path is the file, relative to the root.
	Path string
	// Message is the parser's complaint.
	Message string
}

// Result is what a scan found.
type Result struct {
	// Suppressions are in path, then line, order.
	Suppressions []Suppression
	// Unparseable are the files that could not be read as what their name says.
	Unparseable []Unparseable
}

// Options narrow a scan.
type Options struct {
	// Scope limits the scan to these paths (files or directories) relative to the root; empty
	// means the whole project.
	Scope []string
	// Exclude lists globs, relative to the root, of files and directories not scanned.
	Exclude []string
}

// skippedDirs are never walked: the go tool ignores testdata, and the others hold no project code.
var skippedDirs = []string{".git", "vendor", "testdata", "node_modules"}

// Scan walks the project at dir and returns every suppression and every unparseable file.
func Scan(dir string, opts Options) (Result, error) {
	handle, err := os.OpenRoot(dir)
	if err != nil {
		return Result{}, fmt.Errorf("scan %s: %w", dir, err)
	}
	fsys := handle.FS()
	var result Result
	walkErr := fs.WalkDir(fsys, ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if skipped(name, entry, opts.Exclude) {
			return skipDir(entry)
		}
		if entry.IsDir() || !entry.Type().IsRegular() || !root.InScope(name, opts.Scope) {
			return nil
		}
		scanner, ok := scannerFor(name)
		if !ok {
			return nil
		}
		data, err := fs.ReadFile(fsys, name)
		if err != nil {
			return fmt.Errorf("reading %s: %w", name, err)
		}
		scanned := scanner(name, data)
		if scanned.problem != "" {
			result.Unparseable = append(result.Unparseable, Unparseable{Path: name, Message: scanned.problem})
			return nil
		}
		result.Suppressions = append(result.Suppressions, scanned.found...)
		return nil
	})
	closeErr := handle.Close()
	if walkErr != nil {
		return Result{}, fmt.Errorf("scan %s: %w", dir, walkErr)
	}
	if closeErr != nil {
		return Result{}, fmt.Errorf("scan %s: %w", dir, closeErr)
	}
	sortResult(&result)
	return result, nil
}

func skipDir(entry fs.DirEntry) error {
	if entry.IsDir() {
		return fs.SkipDir
	}
	return nil
}

// skipped reports whether the walk leaves name alone: a directory that holds no project code, or
// anything an exclude glob names. The root itself is never skipped.
func skipped(name string, entry fs.DirEntry, exclude []string) bool {
	if name == "." {
		return false
	}
	if entry.IsDir() && slices.Contains(skippedDirs, entry.Name()) {
		return true
	}
	for _, pattern := range exclude {
		if ok, err := glob.Match(pattern, name); err == nil && ok {
			return true
		}
	}
	return false
}

// fileScan is what reading one file yields: its suppressions, or the parser's complaint when the
// file cannot be read as what its name says. An unparseable file is a result of the scan, not a
// failure of it.
type fileScan struct {
	found   []Suppression
	problem string
}

func unparseable(err error) fileScan { return fileScan{problem: err.Error()} }

// fileScanner extracts the suppressions from one file.
type fileScanner func(name string, data []byte) fileScan

// scannerFor picks the scanner for a file by its name; false means the file holds no route.
func scannerFor(name string) (fileScanner, bool) {
	base := path.Base(name)
	switch {
	case strings.HasSuffix(base, ".go"):
		return scanGo, true
	case base == "staticcheck.conf":
		return scanStaticcheckConf, true
	case isGolangciConfig(base):
		return scanGolangci, true
	default:
		return nil, false
	}
}

func sortResult(result *Result) {
	slices.SortStableFunc(result.Suppressions, func(a, b Suppression) int {
		if c := strings.Compare(a.Path, b.Path); c != 0 {
			return c
		}
		return a.Line - b.Line
	})
	slices.SortFunc(result.Unparseable, func(a, b Unparseable) int { return strings.Compare(a.Path, b.Path) })
}
