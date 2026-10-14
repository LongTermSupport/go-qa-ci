package runners

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/LongTermSupport/go-qa-ci/internal/finding"
)

const (
	govulncheck = "govulncheck"
	// modFile is the location of a vulnerability finding that has no call site in the code.
	modFile = "go.mod"
)

// message is one object of govulncheck's -format json stream; only the parts used are decoded.
type message struct {
	OSV     *osvEntry    `json:"osv"`
	Finding *vulnFinding `json:"finding"`
}

type osvEntry struct {
	ID      string `json:"id"`
	Summary string `json:"summary"`
}

type vulnFinding struct {
	OSV          string  `json:"osv"`
	FixedVersion string  `json:"fixed_version"`
	Trace        []frame `json:"trace"`
}

type frame struct {
	Module   string `json:"module"`
	Version  string `json:"version"`
	Package  string `json:"package"`
	Function string `json:"function"`
	Position *struct {
		Filename string `json:"filename"`
		Line     int    `json:"line"`
	} `json:"position"`
}

// reached reports whether the finding is symbol level: the vulnerable function is called.
func (f vulnFinding) reached() bool {
	return len(f.Trace) > 0 && f.Trace[0].Function != ""
}

// ParseVuln reads a govulncheck -format json stream and returns one finding per vulnerability,
// module and call-site file whose vulnerable symbol the code calls, so that a record exception
// scoped to one file cannot license the advisory in another. Vulnerabilities that are only present in a
// dependency, or in a package that is imported but not called, are not findings: the scan
// level is symbol, so reachability decides (TOOLING-SPEC 6).
func ParseVuln(r io.Reader) ([]finding.Finding, error) {
	decoder := json.NewDecoder(r)
	summaries := map[string]string{}
	var reached []vulnFinding
	for {
		var msg message
		err := decoder.Decode(&msg)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading %s output: %w", govulncheck, err)
		}
		if msg.OSV != nil {
			summaries[msg.OSV.ID] = msg.OSV.Summary
		}
		if msg.Finding != nil && msg.Finding.reached() {
			reached = append(reached, *msg.Finding)
		}
	}
	seen := map[string]bool{}
	var findings []finding.Finding
	for _, f := range reached {
		vulnerable := f.Trace[0]
		key := f.OSV + "\x00" + vulnerable.Module + "\x00" + vulnFindingOf(f, "").Path
		if seen[key] {
			continue
		}
		seen[key] = true
		findings = append(findings, vulnFindingOf(f, summaries[f.OSV]))
	}
	return findings, nil
}

func vulnFindingOf(f vulnFinding, summary string) finding.Finding {
	vulnerable := f.Trace[0]
	call := f.Trace[len(f.Trace)-1]
	path, line := modFile, 0
	if call.Position != nil && call.Position.Filename != "" {
		path, line = call.Position.Filename, call.Position.Line
	}
	var message strings.Builder
	if summary != "" {
		message.WriteString(summary + ": ")
	}
	fmt.Fprintf(&message, "%s %s@%s is called through %s", vulnerable.Package, vulnerable.Module, vulnerable.Version, vulnerable.Function)
	if f.FixedVersion != "" {
		fmt.Fprintf(&message, "; fixed in %s", f.FixedVersion)
	}
	return finding.Finding{Rule: f.OSV, Path: path, Line: line, Message: message.String()}
}

// Vuln runs govulncheck over the module in dir and returns the vulnerabilities its code reaches.
func Vuln(ctx context.Context, dir string) ([]finding.Finding, error) {
	if err := Locate(govulncheck); err != nil {
		return nil, err
	}
	out, err := Capture(ctx, dir, govulncheck, "-format", "json", "./...")
	if err != nil {
		return nil, err
	}
	// With -format json govulncheck exits 0 whatever it finds, so a non-zero exit is a failure to scan.
	if out.Code != 0 {
		return nil, fmt.Errorf("%s exited %d: %s", govulncheck, out.Code, strings.TrimSpace(out.Stderr))
	}
	return ParseVuln(strings.NewReader(out.Stdout))
}
