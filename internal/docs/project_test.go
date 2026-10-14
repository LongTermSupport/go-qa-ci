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
