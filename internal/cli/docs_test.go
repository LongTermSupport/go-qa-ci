package cli_test

import (
	"strings"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/builtin"
)

func TestRuleDocPrintsABundledPage(t *testing.T) {
	r := exec("--root", module(t, ""), "rule-doc", builtin.RecordStale)
	if r.code != 0 || !strings.HasPrefix(r.out, "# "+builtin.RecordStale+"\n") || !strings.Contains(r.out, "## How to fix correctly") {
		t.Errorf("got %+v", r)
	}
	if !strings.Contains(r.err, "source: bundled") {
		t.Errorf("the source belongs on stderr: %q", r.err)
	}
}

func TestRuleDocResolvesAProjectPageByItsIdentifier(t *testing.T) {
	root := module(t, "")
	writeFile(t, root, "docs/defences/example.rule.md", "# example.rule\n\n## How to fix correctly\n\nWrite it this way.\n")
	r := exec("--root", root, "rule-doc", "example.rule")
	if r.code != 0 || !strings.Contains(r.out, "Write it this way.") || !strings.Contains(r.err, "source: project") {
		t.Errorf("got %+v", r)
	}
}

func TestRuleDocHonoursAConfiguredDocsDir(t *testing.T) {
	root := module(t, "docs_dir = \"pages\"\n")
	writeFile(t, root, "pages/example.rule.md", "# example.rule\n\nconfigured\n")
	r := exec("--root", root, "rule-doc", "example.rule")
	if r.code != 0 || !strings.Contains(r.out, "configured") {
		t.Errorf("got %+v", r)
	}
}

func TestRuleDocUnknownIdentifierExitsOne(t *testing.T) {
	r := exec("--root", module(t, ""), "rule-doc", "no.such.rule")
	if r.code != 1 || r.out != "" || !strings.Contains(r.err, "no.such.rule") || !strings.Contains(r.err, "dangling") {
		t.Errorf("got %+v", r)
	}
}

func TestRuleDocNeedsExactlyOneIdentifier(t *testing.T) {
	for _, args := range [][]string{{"rule-doc"}, {"rule-doc", "a", "b"}} {
		r := exec(append([]string{"--root", module(t, "")}, args...)...)
		if r.code != 2 || !strings.Contains(r.err, "rule-doc") {
			t.Errorf("%v: got %+v", args, r)
		}
	}
}

func TestDocsCheckPassesOnACleanProject(t *testing.T) {
	r := exec("--root", module(t, ""), "docs-check")
	if r.code != 0 || r.err != "" || !strings.Contains(r.out, "docs:") {
		t.Errorf("got %+v", r)
	}
}

func TestDocsCheckFailsOnAnOrphanPage(t *testing.T) {
	root := module(t, "")
	writeFile(t, root, "docs/defences/orphaned.rule.md", "# orphaned.rule\n")
	r := exec("--root", root, "docs-check")
	if r.code != 1 || !strings.Contains(r.err, "error[gqaci.docs.orphan]") || !strings.Contains(r.err, "docs/defences/orphaned.rule.md") {
		t.Errorf("got %+v", r)
	}
}

func TestDocsCheckAcceptsAPageForARecordRule(t *testing.T) {
	root := recordModule(t, validRecord())
	writeFile(t, root, "docs/defences/errcheck.md", "# errcheck\n")
	if r := exec("--root", root, "docs-check"); r.code != 0 {
		t.Errorf("got %+v", r)
	}
}

func TestDocsCheckRejectsArguments(t *testing.T) {
	r := exec("--root", module(t, ""), "docs-check", "extra")
	if r.code != 2 {
		t.Errorf("got %+v", r)
	}
}

const projectRuleYAML = "id: no-println\nlanguage: go\nseverity: error\nmessage: no println\nrule:\n  pattern: println($$$A)\n"

func projectRuleModule(t *testing.T) string {
	t.Helper()
	root := module(t, "")
	writeFile(t, root, "sgconfig.yml", "ruleDirs:\n  - rules\n")
	writeFile(t, root, "rules/no-println.yml", projectRuleYAML)
	return root
}

func TestDocsCheckReportsAProjectAstGrepRuleWithoutAPageAsDangling(t *testing.T) {
	root := projectRuleModule(t)
	check := exec("--root", root, "docs-check")
	if check.code != 1 || !strings.Contains(check.err, "gqaci.docs.dangling") || !strings.Contains(check.err, "no-println") {
		t.Errorf("docs-check: %+v", check)
	}
	if doc := exec("--root", root, "rule-doc", "no-println"); doc.code != 1 {
		t.Errorf("rule-doc and docs-check must agree that the rule is dangling: %+v", doc)
	}
}

func TestDocsCheckAcceptsTheProjectAstGrepRulePageRuleDocNames(t *testing.T) {
	root := projectRuleModule(t)
	writeFile(t, root, "docs/defences/no-println.md", "# no-println\n\n## How to fix correctly\n\nUse the logger.\n")
	if check := exec("--root", root, "docs-check"); check.code != 0 {
		t.Errorf("docs-check: %+v", check)
	}
	if doc := exec("--root", root, "rule-doc", "no-println"); doc.code != 0 || !strings.Contains(doc.out, "Use the logger.") {
		t.Errorf("rule-doc: %+v", doc)
	}
}
