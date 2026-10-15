package tools

import (
	"slices"

	"github.com/LongTermSupport/go-qa-ci/internal/config"
	"github.com/LongTermSupport/go-qa-ci/internal/pipeline"
)

// stepTools maps a step to the pinned external tool its engine launches.
var stepTools = map[string]string{
	"fmt":      "golangci-lint",
	"golangci": "golangci-lint",
	"ast-grep": "ast-grep",
	"deadcode": "deadcode",
	"vuln":     "govulncheck",
	"mutants":  "gremlins",
}

// Needs are the pinned external tools the steps launch, sorted and without repeats. The summary
// step lists the golangci linters, so it needs golangci-lint when that tool is switched on.
func Needs(steps []pipeline.Step, cfg config.Config) []string {
	var needed []string
	for _, step := range steps {
		name, ok := stepTools[step.Name]
		if step.Name == "summary" && cfg.Tools.Golangci {
			name, ok = "golangci-lint", true
		}
		if ok && !slices.Contains(needed, name) {
			needed = append(needed, name)
		}
	}
	slices.Sort(needed)
	return needed
}
