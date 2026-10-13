package cli

import (
	"io"

	"github.com/LongTermSupport/go-qa-ci/internal/config"
	"github.com/LongTermSupport/go-qa-ci/internal/docs"
)

// runRuleDoc prints the page for one identifier; a dangling identifier is a failure.
func runRuleDoc(rootPath string, cfg config.Config, args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		return emit(stderr, ExitUsage, "go-qa rule-doc: want exactly one identifier\nusage: go-qa rule-doc <id>")
	}
	doc, found, err := docs.Resolver{Root: rootPath, DocsDir: cfg.DocsDir}.Resolve(args[0])
	if err != nil {
		return emit(stderr, ExitFailure, "go-qa: %v", err)
	}
	if !found {
		return emit(stderr, ExitFailure, "no documentation for `%s` (dangling identifier); a project rule's page is %s/%s",
			args[0], cfg.DocsDir, docs.FileName(args[0]))
	}
	if code := emit(stdout, ExitOK, "%s", doc.Text); code != ExitOK {
		return code
	}
	return emit(stderr, ExitOK, "(source: %s)", doc.Source)
}

// runDocsCheck fails when an identifier is dangling, a page is an orphan or a bundled page is
// malformed.
func runDocsCheck(rootPath string, cfg config.Config, args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		return emit(stderr, ExitUsage, "go-qa docs-check: unexpected argument %q", args[0])
	}
	findings, err := docs.Run(rootPath, cfg)
	if err != nil {
		return emit(stderr, ExitFailure, "go-qa: %v", err)
	}
	for _, f := range findings {
		if code := emit(stderr, ExitFailure, "%s", f); code != ExitFailure {
			return code
		}
	}
	if len(findings) > 0 {
		return ExitFailure
	}
	return emit(stdout, ExitOK, "docs: every identifier resolves and no page is an orphan")
}
