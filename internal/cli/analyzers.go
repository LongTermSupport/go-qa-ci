package cli

import (
	"context"
	"fmt"
	"slices"

	"github.com/LongTermSupport/go-qa-ci/internal/config"
	"github.com/LongTermSupport/go-qa-ci/internal/vet"
)

// analyzerSource names where an analyzer's documentation comes from.
const analyzerSource = "analyzer help"

// analyzerDoc returns the documentation a vet tool prints for id when id is one of its analyzers.
func analyzerDoc(rootPath string, cfg config.Config, id string) (text string, found bool, err error) {
	ctx := context.Background()
	err = vet.WithAnalyzers(ctx, cfg.Analyzers.BuildDir(rootPath), cfg.Analyzers.Driver, func(tool vet.Tool, analyzers []vet.Analyzer) error {
		if !slices.ContainsFunc(analyzers, func(a vet.Analyzer) bool { return a.Name == id }) {
			return nil
		}
		help, helpErr := tool.Help(ctx, id)
		if helpErr != nil {
			return fmt.Errorf("documentation of %s: %w", id, helpErr)
		}
		text, found = help, true
		return nil
	})
	if err != nil {
		return "", false, fmt.Errorf("looking up %s among the analyzers: %w", id, err)
	}
	return text, found, nil
}
