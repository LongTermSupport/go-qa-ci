package docs

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/LongTermSupport/go-qa-ci/internal/config"
	"github.com/LongTermSupport/go-qa-ci/internal/finding"
	"github.com/LongTermSupport/go-qa-ci/internal/record"
)

// Run is the docs detector for the project at root as cfg describes it. The rules of the project
// record are identifiers the project can see, so a page for one of them is not an orphan; they
// are not required to have a page, because a third-party detector's catalogue resolves under its
// own clause (TOOLING-SPEC 4.2). Until the detector takes its identifiers from the defence listing, a page for a project rule
// that is neither a built-in nor a record rule is reported as an orphan: a recorded gap.
func Run(ctx context.Context, root string, cfg config.Config) ([]finding.Finding, error) {
	rec, err := record.Load(filepath.Join(root, filepath.FromSlash(cfg.Record.Path)))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", cfg.Record.Path, err)
	}
	documented := make([]string, 0, len(rec.Exceptions))
	for _, exception := range rec.Exceptions {
		documented = append(documented, exception.Rule)
	}
	return Check(ctx, Input{Root: root, DocsDir: cfg.DocsDir, Documented: documented, Stages: cfg.Stages, Patterns: cfg.Resolvers})
}
