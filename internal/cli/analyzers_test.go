package cli_test

import (
	"os"
	"strings"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/vettest"
)

// TestMain lets this test binary stand in for go-qa as the go vet tool, as the binary does in
// production when the analyzers are listed or documented.
func TestMain(m *testing.M) {
	vettest.Serve()
	os.Exit(m.Run())
}

func TestRulesListsTheAnalyzersWithTheirSummary(t *testing.T) {
	r := exec("--root", module(t, ""), "rules")
	want := "analyzer  gqaci_test  reports calls to a function named Forbidden\n"
	if r.code != 0 || r.out != want {
		t.Errorf("got %+v, want stdout %q", r, want)
	}
}

func TestRulesTakesNoArguments(t *testing.T) {
	r := exec("--root", module(t, ""), "rules", "extra")
	if r.code != 2 || !strings.Contains(r.err, "rules") {
		t.Errorf("got %+v", r)
	}
}

func TestRuleDocPrintsAnAnalyzersDocumentationFromTheDriver(t *testing.T) {
	r := exec("--root", module(t, ""), "rule-doc", "gqaci_test")
	if r.code != 0 || !strings.Contains(r.out, "the fix is to call something else") || !strings.Contains(r.err, "source: analyzer help") {
		t.Errorf("got %+v", r)
	}
}

func TestRuleDocPrefersAProjectPageOverTheAnalyzerHelp(t *testing.T) {
	root := module(t, "")
	writeFile(t, root, "docs/defences/gqaci_test.md", "# gqaci_test\n\nproject words\n")
	r := exec("--root", root, "rule-doc", "gqaci_test")
	if r.code != 0 || !strings.Contains(r.out, "project words") || !strings.Contains(r.err, "source: project") {
		t.Errorf("got %+v", r)
	}
}
