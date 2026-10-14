package docs_test

import (
	"slices"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/config"
	"github.com/LongTermSupport/go-qa-ci/internal/docs"
	"github.com/LongTermSupport/go-qa-ci/internal/finding"
)

func TestRunTreatsRecordRulesAsDocumentable(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	record := "[[exception]]\nrule = \"errcheck\"\npath = \"a.go\"\njustification = \"x\"\n"
	write(t, root, "qa/record.toml", record)
	write(t, root, docsDir+"/errcheck.md", page("errcheck"))
	write(t, root, docsDir+"/gone.md", page("gone"))
	findings, err := docs.Run(t.Context(), root, config.Default())
	if err != nil {
		t.Fatal(err)
	}
	var orphans []string
	for _, f := range findings {
		if f.Rule == finding.DocsOrphan {
			orphans = append(orphans, f.Path)
		}
	}
	if !slices.Equal(orphans, []string{docsDir + "/gone.md"}) {
		t.Errorf("findings = %v", findings)
	}
}

func TestRunMalformedRecordIsAnError(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write(t, root, "qa/record.toml", "[[exception]]\nreson = 1\n")
	if _, err := docs.Run(t.Context(), root, config.Default()); err == nil {
		t.Fatal("a record that cannot be read must not be treated as empty")
	}
}

const sgRule = "id: no-println\nlanguage: go\nseverity: error\nmessage: no println\nrule:\n  pattern: println($$$A)\n"

func writeSgProject(t *testing.T, root string) {
	t.Helper()
	write(t, root, "sgconfig.yml", "ruleDirs:\n  - rules\n")
	write(t, root, "rules/no-println.yml", sgRule)
}

func docsFindings(t *testing.T, root string, rule string) []string {
	t.Helper()
	findings, err := docs.Run(t.Context(), root, config.Default())
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, f := range findings {
		if f.Rule == rule {
			paths = append(paths, f.Path)
		}
	}
	return paths
}

func TestRunReportsAProjectAstGrepRuleWithoutAPageAsDangling(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSgProject(t, root)
	got := docsFindings(t, root, finding.DocsDangling)
	if !slices.Equal(got, []string{docsDir + "/no-println.md"}) {
		t.Errorf("dangling = %v", got)
	}
}

func TestRunAcceptsTheProjectAstGrepRulePage(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSgProject(t, root)
	write(t, root, docsDir+"/no-println.md", page("no-println"))
	if got := docsFindings(t, root, finding.DocsDangling); len(got) > 0 {
		t.Errorf("dangling = %v", got)
	}
	if got := docsFindings(t, root, finding.DocsOrphan); len(got) > 0 {
		t.Errorf("orphan = %v", got)
	}
}

func TestRunIgnoresProjectAstGrepRulesWhenTheToolIsOff(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSgProject(t, root)
	cfg := config.Default()
	cfg.Tools.AstGrep = false
	findings, err := docs.Run(t.Context(), root, cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range findings {
		if f.Rule == finding.DocsDangling && f.Path == docsDir+"/no-println.md" {
			t.Errorf("a disabled tool's rule was judged: %v", f)
		}
	}
}
