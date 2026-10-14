package tools_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/config"
	"github.com/LongTermSupport/go-qa-ci/internal/pipeline"
	"github.com/LongTermSupport/go-qa-ci/internal/tools"
)

// The directive is assembled so that no guard reading raw text mistakes the fixture for a real one.
const disableDirective = "//" + "no" + "lint"

const suppressedSource = "package p\n\nfunc g() {\n\tf() " + disableDirective + ":errcheck // the error is accounted for\n}\n"

const errcheckException = "[[exception]]\nrule = \"errcheck\"\npath = \"p/p.go\"\njustification = \"the error is accounted for in this one file only\"\n"

func suppressionRoot(t *testing.T, files map[string]string) tools.Context {
	t.Helper()
	root := t.TempDir()
	for name, text := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return tools.Context{Root: root}
}

func TestSuppressionFailsAnUnrecordedDirectiveAndPrintsTheFinding(t *testing.T) {
	ctx := suppressionRoot(t, map[string]string{"p/p.go": suppressedSource})
	outcome, stderr := runStep(t, ctx, "suppression")
	if outcome.Kind != pipeline.KindFailed || !strings.Contains(stderr, "error[gqaci.suppression.unrecorded]") || !strings.Contains(stderr, "--> p/p.go:4") {
		t.Errorf("got %+v\n%s", outcome, stderr)
	}
}

func TestSuppressionPassesARecordedDirective(t *testing.T) {
	ctx := suppressionRoot(t, map[string]string{"p/p.go": suppressedSource, "qa/record.toml": errcheckException})
	outcome, stderr := runStep(t, ctx, "suppression")
	if outcome.Kind != pipeline.KindPassed || stderr != "" {
		t.Errorf("got %+v\n%s", outcome, stderr)
	}
}

func TestSuppressionFailsAnUnparseableFile(t *testing.T) {
	ctx := suppressionRoot(t, map[string]string{"p/p.go": "package p\nfunc ("})
	outcome, stderr := runStep(t, ctx, "suppression")
	if outcome.Kind != pipeline.KindFailed || !strings.Contains(stderr, "cannot parse") {
		t.Errorf("got %+v\n%s", outcome, stderr)
	}
}

func TestSuppressionReportsAStaleExceptionOnAFullRunOnly(t *testing.T) {
	files := map[string]string{"p/p.go": "package p\n", "qa/record.toml": errcheckException}
	outcome, stderr := runStep(t, suppressionRoot(t, files), "suppression")
	if outcome.Kind != pipeline.KindFailed || !strings.Contains(stderr, "error[gqaci.record.stale]") {
		t.Fatalf("full run: %+v\n%s", outcome, stderr)
	}
	scoped := suppressionRoot(t, files)
	scoped.Scope = []string{"p"}
	outcome, stderr = runStep(t, scoped, "suppression")
	if outcome.Kind != pipeline.KindPassed || stderr != "" {
		t.Errorf("a scoped run cannot judge staleness: %+v\n%s", outcome, stderr)
	}
}

func TestSuppressionHonoursScanExclude(t *testing.T) {
	ctx := suppressionRoot(t, map[string]string{"p/p.go": suppressedSource})
	ctx.Config = config.Default()
	ctx.Config.ScanExclude = []string{"p"}
	outcome, stderr := runStep(t, ctx, "suppression")
	if outcome.Kind != pipeline.KindPassed || stderr != "" {
		t.Errorf("got %+v\n%s", outcome, stderr)
	}
}
