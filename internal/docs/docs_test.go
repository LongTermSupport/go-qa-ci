package docs_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/builtin"
	"github.com/LongTermSupport/go-qa-ci/internal/docs"
	"github.com/LongTermSupport/go-qa-ci/internal/finding"
)

const docsDir = "docs/defences"

func write(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func page(id string) string {
	return "# " + id + "\n\n## How to fix correctly\n\nWrite it this way.\n"
}

func TestEveryBuiltinHasAPageWithTheRequiredShape(t *testing.T) {
	t.Parallel()
	resolver := docs.Resolver{Root: t.TempDir(), DocsDir: docsDir}
	for _, id := range builtin.All() {
		doc, found, err := resolver.Resolve(id)
		if err != nil || !found {
			t.Errorf("%s: found=%v err=%v", id, found, err)
			continue
		}
		if doc.Source != docs.Bundled {
			t.Errorf("%s: source %v, want bundled", id, doc.Source)
		}
		if first, _, _ := strings.Cut(doc.Text, "\n"); first != "# "+id {
			t.Errorf("%s: first line %q", id, first)
		}
		if !strings.Contains(doc.Text, "\n## How to fix correctly\n") {
			t.Errorf("%s: no How to fix correctly section", id)
		}
	}
}

func TestAuditBundledIsCleanForTheShippedPages(t *testing.T) {
	t.Parallel()
	if findings := docs.AuditBundled(); len(findings) > 0 {
		t.Errorf("the shipped pages fail their own audit: %v", findings)
	}
}

func TestProjectPageResolvedByIdentifierAndWinsOverBundled(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write(t, root, docsDir+"/example.rule.md", page("example.rule"))
	write(t, root, docsDir+"/"+builtin.RecordStale+".md", "# "+builtin.RecordStale+"\n\nproject text\n")
	resolver := docs.Resolver{Root: root, DocsDir: docsDir}

	doc, found, err := resolver.Resolve("example.rule")
	if err != nil || !found || doc.Source != docs.Project || doc.Path != docsDir+"/example.rule.md" {
		t.Fatalf("project page: %+v found=%v err=%v", doc, found, err)
	}
	doc, found, err = resolver.Resolve(builtin.RecordStale)
	if err != nil || !found || doc.Source != docs.Project || !strings.Contains(doc.Text, "project text") {
		t.Fatalf("project page must win over the bundled one: %+v found=%v err=%v", doc, found, err)
	}
}

func TestProjectPageFoundUnderAnotherSpellingOfTheName(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write(t, root, docsDir+"/Gqaci-Test-Rule.md", page("gqaci_test_rule"))
	resolver := docs.Resolver{Root: root, DocsDir: docsDir}
	if _, found, err := resolver.Resolve("gqaci_test_rule"); err != nil || !found {
		t.Fatalf("found=%v err=%v", found, err)
	}
}

func TestResolveUnknownAndUnsafeIdentifiers(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write(t, root, "secret.md", "outside the docs directory")
	resolver := docs.Resolver{Root: root, DocsDir: docsDir}
	for _, id := range []string{"no.such.rule", "", "  ", "../../secret", "a/b", `a\b`, ".."} {
		if doc, found, err := resolver.Resolve(id); err != nil || found {
			t.Errorf("Resolve(%q) = %+v found=%v err=%v", id, doc, found, err)
		}
	}
}

func TestResolveTrimsTheIdentifier(t *testing.T) {
	t.Parallel()
	resolver := docs.Resolver{Root: t.TempDir(), DocsDir: docsDir}
	if _, found, err := resolver.Resolve(" " + builtin.DocsOrphan + "\n"); err != nil || !found {
		t.Fatalf("found=%v err=%v", found, err)
	}
}

func TestResolveFailsLoudlyWhenAProjectPageCannotBeRead(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// A directory named like a page: it exists but is not a readable page.
	if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(docsDir), "example.rule.md"), 0o750); err != nil {
		t.Fatal(err)
	}
	resolver := docs.Resolver{Root: root, DocsDir: docsDir}
	if _, _, err := resolver.Resolve("example.rule"); err == nil {
		t.Fatal("an unreadable project page must be an error, not a fall-through to a later source")
	}
}

func TestOrphans(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write(t, root, docsDir+"/README.md", "index")
	write(t, root, docsDir+"/example.kept.md", page("example.kept"))
	write(t, root, docsDir+"/Example-Alias.md", page("example_alias"))
	write(t, root, docsDir+"/removed.rule.md", page("removed.rule"))
	write(t, root, docsDir+"/notes.txt", "not a page")
	got, err := docs.Orphans(root, docsDir, []string{"example.kept", "example_alias"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != docsDir+"/removed.rule.md" {
		t.Errorf("orphans = %v", got)
	}
}

func TestOrphansMissingDirectoryHoldsNone(t *testing.T) {
	t.Parallel()
	got, err := docs.Orphans(t.TempDir(), docsDir, nil)
	if err != nil || len(got) != 0 {
		t.Errorf("orphans = %v err=%v", got, err)
	}
}

func TestCheckCleanProject(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write(t, root, docsDir+"/example.rule.md", page("example.rule"))
	findings, err := docs.Check(docs.Input{Root: root, DocsDir: docsDir, Listed: []string{"example.rule"}})
	if err != nil || len(findings) > 0 {
		t.Fatalf("findings=%v err=%v", findings, err)
	}
}

func TestCheckOrphanPageFails(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write(t, root, docsDir+"/orphaned.rule.md", page("orphaned.rule"))
	findings, err := docs.Check(docs.Input{Root: root, DocsDir: docsDir})
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 || findings[0].Rule != finding.DocsOrphan || findings[0].Path != docsDir+"/orphaned.rule.md" {
		t.Fatalf("findings = %v", findings)
	}
}

func TestCheckPageOfADocumentedRuleIsNotAnOrphan(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write(t, root, docsDir+"/example.recorded.md", page("example.recorded"))
	findings, err := docs.Check(docs.Input{Root: root, DocsDir: docsDir, Documented: []string{"example.recorded"}})
	if err != nil || len(findings) > 0 {
		t.Fatalf("findings=%v err=%v", findings, err)
	}
}

func TestCheckListedIdentifierWithoutAPageIsDangling(t *testing.T) {
	t.Parallel()
	findings, err := docs.Check(docs.Input{Root: t.TempDir(), DocsDir: docsDir, Listed: []string{"example.rule"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 || findings[0].Rule != finding.DocsDangling || findings[0].Path != docsDir+"/example.rule.md" {
		t.Fatalf("findings = %v", findings)
	}
	if !strings.Contains(findings[0].Message, "example.rule") {
		t.Errorf("message does not name the identifier: %q", findings[0].Message)
	}
}

func TestCheckReportsInDeterministicOrder(t *testing.T) {
	t.Parallel()
	in := docs.Input{Root: t.TempDir(), DocsDir: docsDir, Listed: []string{"z.rule", "a.rule", "a.rule"}}
	findings, err := docs.Check(in)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 2 || findings[0].Path != docsDir+"/a.rule.md" || findings[1].Path != docsDir+"/z.rule.md" {
		t.Fatalf("findings = %v", findings)
	}
}
