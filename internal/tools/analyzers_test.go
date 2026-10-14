package tools_test

import (
	"os"
	"strings"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/astgreptest"
	"github.com/LongTermSupport/go-qa-ci/internal/config"
	"github.com/LongTermSupport/go-qa-ci/internal/pipeline"
	"github.com/LongTermSupport/go-qa-ci/internal/tools"
	"github.com/LongTermSupport/go-qa-ci/internal/vettest"
)

// TestMain lets this test binary stand in for go-qa as the go vet tool, which is exactly how the
// analyzers detector reaches the bundled analyzers in production.
func TestMain(m *testing.M) {
	vettest.Serve()
	astgreptest.Serve()
	os.Exit(m.Run())
}

const (
	forbiddenSource = "// Package p is a fixture.\npackage p\n\n// Forbidden is forbidden.\nfunc Forbidden() {}\n\n// F calls it.\nfunc F() { Forbidden() }\n"
	cleanSource     = "// Package p is a fixture.\npackage p\n\n// F is clean.\nfunc F() {}\n"
	findingsRecord  = "[[exception]]\nrule = \"gqaci_test\"\npath = \"p/p.go\"\ncovers = \"findings\"\njustification = \"the fixture calls the forbidden function on purpose\"\n"
)

func TestAnalyzersFailsAFindingWithTheAnalyzerNameAsIdentifier(t *testing.T) {
	root := fixture(t, map[string]string{"p/p.go": forbiddenSource})
	outcome, stderr := runStep(t, tools.Context{Root: root}, "analyzers")
	if outcome.Kind != pipeline.KindFailed {
		t.Fatalf("got %+v", outcome)
	}
	if !strings.Contains(stderr, "error[gqaci_test]: call to Forbidden") || !strings.Contains(stderr, "--> p/p.go:8") {
		t.Errorf("stderr:\n%s", stderr)
	}
}

func TestAnalyzersPassesACleanModule(t *testing.T) {
	root := fixture(t, map[string]string{"p/p.go": cleanSource})
	outcome, stderr := runStep(t, tools.Context{Root: root}, "analyzers")
	if outcome.Kind != pipeline.KindPassed || stderr != "" {
		t.Errorf("got %+v\n%s", outcome, stderr)
	}
}

func TestAnalyzersPassesAFindingTheRecordLicenses(t *testing.T) {
	root := fixture(t, map[string]string{"p/p.go": forbiddenSource})
	writeRecord(t, root, findingsRecord)
	outcome, stderr := runStep(t, tools.Context{Root: root}, "analyzers")
	if outcome.Kind != pipeline.KindPassed || stderr != "" {
		t.Errorf("got %+v\n%s", outcome, stderr)
	}
}

func TestAnalyzersDoesNotLetASuppressionEntryLicenseAFinding(t *testing.T) {
	root := fixture(t, map[string]string{"p/p.go": forbiddenSource})
	writeRecord(t, root, strings.Replace(findingsRecord, "covers = \"findings\"\n", "", 1))
	outcome, stderr := runStep(t, tools.Context{Root: root}, "analyzers")
	if outcome.Kind != pipeline.KindFailed || !strings.Contains(stderr, "error[gqaci_test]") {
		t.Errorf("got %+v\n%s", outcome, stderr)
	}
}

func TestAnalyzersFailsAnUnusedFindingsExceptionOnAFullRunOnly(t *testing.T) {
	files := map[string]string{"p/p.go": cleanSource}
	root := fixture(t, files)
	writeRecord(t, root, findingsRecord)
	outcome, stderr := runStep(t, tools.Context{Root: root}, "analyzers")
	if outcome.Kind != pipeline.KindFailed || !strings.Contains(stderr, "error[gqaci.record.stale]") {
		t.Fatalf("full run: %+v\n%s", outcome, stderr)
	}
	outcome, stderr = runStep(t, tools.Context{Root: root, Scope: []string{"p"}}, "analyzers")
	if outcome.Kind != pipeline.KindPassed || stderr != "" {
		t.Errorf("a scoped run cannot judge staleness: %+v\n%s", outcome, stderr)
	}
}

func TestAnalyzersLeavesTheStalenessOfOtherDetectorsExceptionsToThem(t *testing.T) {
	root := fixture(t, map[string]string{"p/p.go": cleanSource})
	writeRecord(t, root, strings.ReplaceAll(findingsRecord, "gqaci_test", "errcheck"))
	outcome, stderr := runStep(t, tools.Context{Root: root}, "analyzers")
	if outcome.Kind != pipeline.KindPassed || stderr != "" {
		t.Errorf("got %+v\n%s", outcome, stderr)
	}
}

func TestAnalyzersDropsFindingsOutsideTheScope(t *testing.T) {
	root := fixture(t, map[string]string{"p/p.go": forbiddenSource, "q/q.go": cleanSource})
	outcome, stderr := runStep(t, tools.Context{Root: root, Scope: []string{"q"}}, "analyzers")
	if outcome.Kind != pipeline.KindPassed || stderr != "" {
		t.Errorf("got %+v\n%s", outcome, stderr)
	}
}

func TestAnalyzersCoversTheFurtherModules(t *testing.T) {
	root := fixture(t, map[string]string{
		"p/p.go":        cleanSource,
		"nested/go.mod": "module example.com/nested\n\ngo 1.26\n",
		"nested/n/n.go": forbiddenSource,
	})
	cfg := config.Default()
	cfg.Modules = []string{"nested"}
	outcome, stderr := runStep(t, tools.Context{Root: root, Config: cfg}, "analyzers")
	if outcome.Kind != pipeline.KindFailed || !strings.Contains(stderr, "--> nested/n/n.go:8") {
		t.Errorf("got %+v\n%s", outcome, stderr)
	}
}

func TestAnalyzersFailsCodeThatDoesNotCompile(t *testing.T) {
	root := fixture(t, map[string]string{"p/p.go": "package p\n\nfunc F() { undefined() }\n"})
	outcome, stderr := runStep(t, tools.Context{Root: root}, "analyzers")
	if outcome.Kind != pipeline.KindFailed || !strings.Contains(stderr, "undefined") {
		t.Errorf("got %+v\n%s", outcome, stderr)
	}
}

func TestAnalyzersFailsAProjectDriverThatDoesNotBuild(t *testing.T) {
	root := fixture(t, map[string]string{"p/p.go": cleanSource})
	cfg := config.Default()
	cfg.Analyzers.Driver = "./internal/qa/missing"
	outcome, stderr := runStep(t, tools.Context{Root: root, Config: cfg}, "analyzers")
	if outcome.Kind != pipeline.KindFailed || !strings.Contains(stderr, "internal/qa/missing") {
		t.Errorf("got %+v\n%s", outcome, stderr)
	}
}
