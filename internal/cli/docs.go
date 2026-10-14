package cli

import (
	"context"
	"io"
	"strings"

	"github.com/LongTermSupport/go-qa-ci/internal/config"
	"github.com/LongTermSupport/go-qa-ci/internal/docs"
)

// runRuleDoc prints the page for one identifier; a dangling identifier is a failure.
func runRuleDoc(rootPath string, cfg config.Config, args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		return emit(stderr, ExitUsage, "go-qa rule-doc: want exactly one identifier\nusage: go-qa rule-doc <id>")
	}
	doc, found, err := docs.ForConfig(rootPath, cfg).Resolve(context.Background(), args[0])
	if err != nil {
		return emit(stderr, ExitFailure, "go-qa: %v", err)
	}
	if !found {
		return ruleDocFromAnalyzers(rootPath, cfg, args[0], stdout, stderr)
	}
	if code := emit(stdout, ExitOK, "%s", doc.Text); code != ExitOK {
		return code
	}
	return emit(stderr, ExitOK, "(source: %s)", doc.Source)
}

// ruleDocFromAnalyzers is the last place an identifier is looked up: the documentation of the
// analyzers of the project's vet tool. An identifier nothing documents is dangling.
func ruleDocFromAnalyzers(rootPath string, cfg config.Config, id string, stdout, stderr io.Writer) int {
	text, found, err := analyzerDoc(rootPath, cfg, id)
	if err != nil {
		return emit(stderr, ExitFailure, "go-qa: %v", err)
	}
	if found {
		if code := emit(stdout, ExitOK, "%s", strings.TrimRight(text, "\n")); code != ExitOK {
			return code
		}
		return emit(stderr, ExitOK, "(source: %s)", analyzerSource)
	}
	return emit(stderr, ExitFailure, "no documentation for `%s` (dangling identifier); a project rule's page is %s/%s",
		id, cfg.DocsDir, docs.FileName(id))
}

// runDocsCheck fails when an identifier is dangling, a page is an orphan or a bundled page is
// malformed.
func runDocsCheck(rootPath string, cfg config.Config, args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		return emit(stderr, ExitUsage, "go-qa docs-check: unexpected argument %q", args[0])
	}
	findings, err := docs.Run(context.Background(), rootPath, cfg)
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
