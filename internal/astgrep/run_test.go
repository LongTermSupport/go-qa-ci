package astgrep_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/astgrep"
	"github.com/LongTermSupport/go-qa-ci/internal/astgreptest"
)

func TestLocateNamesTheMissingBinary(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := astgrep.Locate()
	if !errors.Is(err, astgrep.ErrNotInstalled) {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(err.Error(), astgrep.Binary) {
		t.Errorf("error does not name the binary: %v", err)
	}
}

func TestScanPassesRuleExcludesAndScopeAndParsesTheMatches(t *testing.T) {
	argsFile := astgreptest.Install(t, scanJSON, 1)
	runner, err := astgrep.Locate()
	if err != nil {
		t.Fatal(err)
	}
	matches, err := runner.Scan(context.Background(), t.TempDir(), astgrep.ScanOptions{
		Rule: "pack.yml", Excludes: []string{"vendor/**", "gen/**"}, Paths: []string{"a", "b"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 2 {
		t.Errorf("matches = %v", matches)
	}
	got := strings.Join(astgreptest.Args(t, argsFile), " ")
	if want := "scan --json=compact --rule pack.yml --globs !vendor/** --globs !gen/** a b"; got != want {
		t.Errorf("args = %q, want %q", got, want)
	}
}

func TestScanWithoutOptionsRunsTheProjectRules(t *testing.T) {
	argsFile := astgreptest.Install(t, "[]", 0)
	runner, err := astgrep.Locate()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runner.Scan(context.Background(), t.TempDir(), astgrep.ScanOptions{}); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(astgreptest.Args(t, argsFile), " "); got != "scan --json=compact" {
		t.Errorf("args = %q", got)
	}
}

func TestScanFailsWhenTheScanDidNotRun(t *testing.T) {
	for name, tc := range map[string]struct {
		out  string
		code int
	}{"exit 2": {"[]", 2}, "not json": {"Error: no such path", 0}} {
		astgreptest.Install(t, tc.out, tc.code)
		runner, err := astgrep.Locate()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := runner.Scan(context.Background(), t.TempDir(), astgrep.ScanOptions{}); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

func TestTestRunsTheRuleTestsAndReportsFailure(t *testing.T) {
	argsFile := astgreptest.Install(t, "test output", 1)
	runner, err := astgrep.Locate()
	if err != nil {
		t.Fatal(err)
	}
	output, passed, err := runner.Test(context.Background(), t.TempDir())
	if err != nil || passed || output != "test output" {
		t.Errorf("output=%q passed=%v err=%v", output, passed, err)
	}
	if got := strings.Join(astgreptest.Args(t, argsFile), " "); got != "test --skip-snapshot-tests" {
		t.Errorf("args = %q", got)
	}
	astgreptest.Install(t, "ok", 0)
	if _, passed, err = runner.Test(context.Background(), t.TempDir()); err != nil || !passed {
		t.Errorf("passed=%v err=%v", passed, err)
	}
}
