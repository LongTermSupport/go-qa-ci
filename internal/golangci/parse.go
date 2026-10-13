// Package golangci runs golangci-lint and turns its JSON report into findings whose identifiers
// are the ones the tool itself prints (TOOLING-SPEC 4.4): the linter name, or linter::code when the
// linter prefixes its message with a stable check code.
package golangci

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/LongTermSupport/go-qa-ci/internal/finding"
)

// codePrefixed lists the linters whose messages start with the code of the check that raised them
// ("SA1019: ...", "G402: ...", "printf: ..."). For these the code is part of the identifier.
var codePrefixed = map[string]bool{
	"staticcheck": true,
	"gosec":       true,
	"govet":       true,
	"gocritic":    true,
	"revive":      true,
}

var codePrefix = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9_-]*): `)

type report struct {
	Issues []issue `json:"Issues"`
}

type issue struct {
	FromLinter string `json:"FromLinter"`
	Text       string `json:"Text"`
	Pos        struct {
		Filename string `json:"Filename"`
		Line     int    `json:"Line"`
	} `json:"Pos"`
}

// Parse converts the JSON report written by `golangci-lint run --output.json.path=stdout`.
func Parse(data []byte) ([]finding.Finding, error) {
	var parsed report
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("golangci-lint JSON report: %w", err)
	}
	findings := make([]finding.Finding, 0, len(parsed.Issues))
	for i, is := range parsed.Issues {
		if is.FromLinter == "" {
			return nil, fmt.Errorf("golangci-lint JSON report: issue %d names no linter", i+1)
		}
		rule, message := identify(is.FromLinter, is.Text)
		findings = append(findings, finding.Finding{
			Rule:    rule,
			Path:    is.Pos.Filename,
			Line:    is.Pos.Line,
			Message: message,
		})
	}
	return findings, nil
}

// identify splits an issue into its identifier and its message.
func identify(linter, text string) (rule, message string) {
	if !codePrefixed[linter] {
		return linter, text
	}
	match := codePrefix.FindStringSubmatch(text)
	if match == nil {
		return linter, text
	}
	return linter + "::" + match[1], strings.TrimPrefix(text, match[0])
}
