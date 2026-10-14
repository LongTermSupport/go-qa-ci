package docs

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"

	"github.com/LongTermSupport/go-qa-ci/internal/astgrep"
	"github.com/LongTermSupport/go-qa-ci/internal/config"
	"github.com/LongTermSupport/go-qa-ci/internal/finding"
	"github.com/LongTermSupport/go-qa-ci/internal/record"
	"github.com/LongTermSupport/go-qa-ci/internal/vet"
)

// Run is the docs detector for the project at root as cfg describes it. The identifiers it
// judges are those the active configuration can print, enumerated the way `go-qa rules` does:
// the built-ins, the [[stage]] identifiers, the rules of the project's ast-grep rule directories
// (read from sgconfig.yml, so ast-grep need not be installed) and, for a project vet driver, its
// analyzers. A project ast-grep rule must have a page; a project analyzer may have one (its help
// text resolves it otherwise). The rules of the project record are identifiers the project can
// see, so a page for one of them is not an orphan; they are not required to have a page, because
// a third-party detector's catalogue resolves under its own clause (TOOLING-SPEC 4.2). A
// golangci-lint linter resolves through golangci-lint itself and is not enumerated here.
func Run(ctx context.Context, root string, cfg config.Config) ([]finding.Finding, error) {
	rec, err := record.Load(filepath.Join(root, filepath.FromSlash(cfg.Record.Path)))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", cfg.Record.Path, err)
	}
	documented := make([]string, 0, len(rec.Exceptions))
	for _, exception := range rec.Exceptions {
		documented = append(documented, exception.Rule)
	}
	listed, err := projectAstGrepRules(root, cfg)
	if err != nil {
		return nil, err
	}
	analyzers, err := projectAnalyzers(ctx, root, cfg)
	if err != nil {
		return nil, err
	}
	return Check(ctx, Input{
		Root: root, DocsDir: cfg.DocsDir, Listed: listed, Documented: slices.Concat(documented, analyzers),
		Stages: cfg.Stages, Patterns: cfg.Resolvers,
	})
}

// projectAstGrepRules are the identifiers of the project's own ast-grep rules, when that tool is
// enabled and the project has an sgconfig.yml.
func projectAstGrepRules(root string, cfg config.Config) ([]string, error) {
	if !cfg.Tools.AstGrep {
		return nil, nil
	}
	project, found, err := astgrep.LoadProject(root)
	if err != nil {
		return nil, fmt.Errorf("reading the ast-grep project: %w", err)
	}
	if !found {
		return nil, nil
	}
	inventory, err := project.Inventory(root)
	if err != nil {
		return nil, fmt.Errorf("listing the ast-grep rules: %w", err)
	}
	ids := make([]string, len(inventory.Rules))
	for i, rule := range inventory.Rules {
		ids[i] = rule.ID
	}
	return ids, nil
}

// projectAnalyzers are the analyzers of the project's own vet driver, when one is configured and
// the tool is enabled.
func projectAnalyzers(ctx context.Context, root string, cfg config.Config) ([]string, error) {
	if !cfg.Tools.Analyzers || cfg.Analyzers.Driver == "" {
		return nil, nil
	}
	var ids []string
	err := vet.WithAnalyzers(ctx, root, cfg.Analyzers.Driver, func(_ vet.Tool, found []vet.Analyzer) error {
		for _, a := range found {
			ids = append(ids, a.Name)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("listing the analyzers: %w", err)
	}
	return ids, nil
}
