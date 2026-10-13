package record_test

import (
	"strings"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/finding"
	"github.com/LongTermSupport/go-qa-ci/internal/record"
)

func TestMatcherFindsByRuleAndPathGlob(t *testing.T) {
	broad := entry("errcheck", "**/y.go", good)
	broad.Policy = true
	rec := record.Record{Exceptions: []record.Exception{broad, entry("govet", "src/a.go", good)}}
	m := rec.Matcher()
	if i, ok := m.Suppression("errcheck", "tests/x/y.go"); !ok || i != 0 {
		t.Errorf("got %d, %v", i, ok)
	}
	if i, ok := m.Suppression("govet", "src/a.go"); !ok || i != 1 {
		t.Errorf("got %d, %v", i, ok)
	}
	if _, ok := m.Suppression("govet", "src/b.go"); ok {
		t.Error("wrong path matched")
	}
	if _, ok := m.Suppression("staticcheck", "tests/x/y.go"); ok {
		t.Error("wrong rule matched")
	}
}

func TestMatcherSeparatesSuppressionsFromFindings(t *testing.T) {
	findings := entry("errcheck", "src/a.go", good)
	findings.Covers = record.CoversFindings
	m := record.Record{Exceptions: []record.Exception{findings}}.Matcher()
	if _, ok := m.Suppression("errcheck", "src/a.go"); ok {
		t.Error("a findings entry licensed a suppression")
	}
	if i, ok := m.Finding("errcheck", "src/a.go"); !ok || i != 0 {
		t.Errorf("got %d, %v", i, ok)
	}
}

func TestStaleReportsEntriesNothingUsed(t *testing.T) {
	rec := record.Record{Exceptions: []record.Exception{
		entry("errcheck", "src/a.go", good),
		entry("govet", "src/b.go", good),
	}}
	m := rec.Matcher()
	if stale := m.Stale("qa/record.toml"); len(stale) != 2 || stale[0].Rule != finding.RecordStale {
		t.Fatalf("nothing used: %v", stale)
	}
	if _, ok := m.Suppression("errcheck", "src/a.go"); !ok {
		t.Fatal("entry not matched")
	}
	stale := m.Stale("qa/record.toml")
	if len(stale) != 1 || !strings.Contains(stale[0].Message, "govet") || !strings.Contains(stale[0].Message, "src/b.go") {
		t.Errorf("got %v", stale)
	}
}
