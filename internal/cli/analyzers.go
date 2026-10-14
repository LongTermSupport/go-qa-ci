package cli

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/LongTermSupport/go-qa-ci/internal/config"
	"github.com/LongTermSupport/go-qa-ci/internal/vet"
)

// analyzerSource names where an analyzer's documentation comes from.
const analyzerSource = "analyzer help"

// withAnalyzers resolves the project's vet tool, lists its analyzers and runs fn with both.
func withAnalyzers(rootPath string, cfg config.Config, fn func(vet.Tool, []vet.Analyzer) error) error {
	ctx := context.Background()
	tool, cleanup, err := vet.Resolve(ctx, rootPath, cfg.Analyzers.Driver)
	if err != nil {
		return fmt.Errorf("resolving the analyzer driver: %w", err)
	}
	defer cleanup()
	analyzers, err := tool.Analyzers(ctx)
	if err != nil {
		return fmt.Errorf("listing the analyzers: %w", err)
	}
	return fn(tool, analyzers)
}

// runRules lists the defences the listing knows so far: the analyzers of the project's vet tool.
// The complete listing (built-ins, linters, stages, runners) is not built yet and is a recorded gap.
func runRules(rootPath string, cfg config.Config, args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		return emit(stderr, ExitUsage, "go-qa rules: unexpected argument %q", args[0])
	}
	var out strings.Builder
	err := withAnalyzers(rootPath, cfg, func(_ vet.Tool, analyzers []vet.Analyzer) error {
		for _, analyzer := range analyzers {
			fmt.Fprintf(&out, "analyzer  %s  %s\n", analyzer.Name, analyzer.Summary)
		}
		return nil
	})
	if err != nil {
		return emit(stderr, ExitFailure, "go-qa: %v", err)
	}
	if _, err := io.WriteString(stdout, out.String()); err != nil {
		return emit(stderr, ExitFailure, "go-qa: %v", err)
	}
	return ExitOK
}

// analyzerDoc returns the documentation a vet tool prints for id when id is one of its analyzers.
func analyzerDoc(rootPath string, cfg config.Config, id string) (text string, found bool, err error) {
	err = withAnalyzers(rootPath, cfg, func(tool vet.Tool, analyzers []vet.Analyzer) error {
		if !slices.ContainsFunc(analyzers, func(a vet.Analyzer) bool { return a.Name == id }) {
			return nil
		}
		help, helpErr := tool.Help(context.Background(), id)
		if helpErr != nil {
			return fmt.Errorf("documentation of %s: %w", id, helpErr)
		}
		text, found = help, true
		return nil
	})
	return text, found, err
}
