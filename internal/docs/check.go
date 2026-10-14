package docs

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"slices"
	"strings"

	"github.com/LongTermSupport/go-qa-ci/internal/builtin"
	"github.com/LongTermSupport/go-qa-ci/internal/config"
	"github.com/LongTermSupport/go-qa-ci/internal/finding"
	"github.com/LongTermSupport/go-qa-ci/internal/stage"
	"github.com/LongTermSupport/go-qa-ci/ruledocs"
)

const (
	readme       = "README"
	bundledDir   = "rules"
	fixHeading   = "## How to fix correctly"
	bundledRoute = "ruledocs/rules/"
)

// Input is what the docs detector checks.
type Input struct {
	// Root is the project root.
	Root string
	// DocsDir holds the project pages, relative to Root with forward slashes.
	DocsDir string
	// Listed are the project's own identifiers that must resolve; the built-ins always must.
	Listed []string
	// Documented are further identifiers a project page may document without them being listed,
	// such as the rules of the project record.
	Documented []string
	// Stages are the project stages: each stage's name and every rule its rules command prints
	// must resolve, and a page for either is not an orphan.
	Stages []stage.Stage
	// Patterns are the configured [[resolver]] entries that document identifiers by glob.
	Patterns []config.Resolver
}

// Orphans returns, sorted, the project pages (relative to root) that document none of ids.
// README.md is never an orphan and a missing directory holds none.
func Orphans(root, docsDir string, ids []string) ([]string, error) {
	handle, err := os.OpenRoot(root)
	if err != nil {
		return nil, fmt.Errorf("open project root: %w", err)
	}
	stems, listErr := pageStems(handle, docsDir)
	if closeErr := handle.Close(); listErr == nil && closeErr != nil {
		return nil, fmt.Errorf("close project root: %w", closeErr)
	}
	if listErr != nil {
		return nil, listErr
	}
	keys := make(map[string]bool, len(ids))
	for _, id := range ids {
		keys[Key(id)] = true
	}
	var found []string
	for _, stem := range stems {
		if stem != readme && !keys[Key(stem)] {
			found = append(found, docsDir+"/"+FileName(stem))
		}
	}
	return found, nil
}

// Check is the docs detector: every built-in and listed identifier resolves, the bundled pages
// pass their audit, and no project page is an orphan.
func Check(ctx context.Context, in Input) ([]finding.Finding, error) {
	resolver := Resolver{Root: in.Root, DocsDir: in.DocsDir, Stages: in.Stages, Patterns: in.Patterns, rules: &stageRules{}}
	stageIDs, err := resolver.stageIdentifiers(ctx)
	if err != nil {
		return nil, err
	}
	must := sortedStrings(slices.Concat(in.Listed, builtin.All(), stageIDs))
	var findings []finding.Finding
	for _, id := range must {
		_, found, resolveErr := resolver.Resolve(ctx, id)
		if resolveErr != nil {
			return nil, resolveErr
		}
		if found {
			continue
		}
		findings = append(findings, finding.Finding{
			Rule:    finding.DocsDangling,
			Path:    in.DocsDir + "/" + FileName(id),
			Message: fmt.Sprintf("identifier %q has no documentation; add a page whose first line is \"# %s\" with a \"How to fix correctly\" section", id, id),
		})
	}
	findings = append(findings, AuditBundled()...)

	orphans, err := Orphans(in.Root, in.DocsDir, append(slices.Clone(must), in.Documented...))
	if err != nil {
		return nil, err
	}
	for _, page := range orphans {
		findings = append(findings, finding.Finding{
			Rule:    finding.DocsOrphan,
			Path:    page,
			Message: "this page documents no identifier the project can see; delete it, rename it to the identifier it documents, or restore the defence",
		})
	}
	return findings, nil
}

// AuditBundled checks the pages shipped with go-qa against the built-in identifiers
// (TOOLING-SPEC 8.1): each built-in has a page, each page belongs to a built-in, begins with
// "# <id>" and has a non-empty "How to fix correctly" section.
func AuditBundled() []finding.Finding {
	return auditPages(ruledocs.FS(), builtin.All())
}

func auditPages(fsys fs.FS, builtins []string) []finding.Finding {
	var findings []finding.Finding
	report := func(id, format string, args ...any) {
		findings = append(findings, finding.Finding{
			Rule:    finding.DocsDangling,
			Path:    bundledRoute + FileName(id),
			Message: fmt.Sprintf(format, args...),
		})
	}
	have := map[string]bool{}
	entries, err := fs.ReadDir(fsys, bundledDir)
	if err != nil {
		report("(directory)", "cannot read the bundled pages: %v", err)
	}
	for _, entry := range entries {
		id := strings.TrimSuffix(entry.Name(), pageSuffix)
		have[id] = true
		if !slices.Contains(builtins, id) {
			report(id, "bundled page %q documents no built-in identifier", id)
			continue
		}
		data, readErr := fs.ReadFile(fsys, bundledDir+"/"+entry.Name())
		if readErr != nil {
			report(id, "cannot read the bundled page: %v", readErr)
			continue
		}
		for _, problem := range pageProblems(id, string(data)) {
			report(id, "%s", problem)
		}
	}
	for _, id := range builtins {
		if !have[id] {
			report(id, "built-in identifier %q has no bundled page", id)
		}
	}
	return findings
}

// pageProblems lists what is wrong with the page text of id.
func pageProblems(id, text string) []string {
	var problems []string
	first := text
	if end := strings.IndexByte(text, '\n'); end >= 0 {
		first = text[:end]
	}
	if first != "# "+id {
		problems = append(problems, fmt.Sprintf("first line is %q, want \"# %s\"", first, id))
	}
	padded := "\n" + text
	start := strings.Index(padded, "\n"+fixHeading+"\n")
	if start < 0 {
		return append(problems, fmt.Sprintf("no %q section stating the correct construction", fixHeading))
	}
	section := padded[start+len(fixHeading)+2:]
	if next := strings.Index("\n"+section, "\n## "); next >= 0 {
		section = section[:max(next-1, 0)]
	}
	if strings.TrimSpace(section) == "" {
		problems = append(problems, fmt.Sprintf("the %q section is empty", fixHeading))
	}
	return problems
}
