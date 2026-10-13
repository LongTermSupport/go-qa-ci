package record_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/finding"
	"github.com/LongTermSupport/go-qa-ci/internal/record"
)

func project(t *testing.T, content string) string {
	t.Helper()
	root := t.TempDir()
	if content != "" {
		path := filepath.Join(root, "qa", "record.toml")
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestCheckReportsFindingsWithTheRelativePath(t *testing.T) {
	root := project(t, "[[exception]]\nrule = \"errcheck\"\npath = \"a.go\"\njustification = \"legacy\"\n")
	rec, findings, err := record.Check(root, "qa/record.toml")
	if err != nil {
		t.Fatal(err)
	}
	if len(rec.Exceptions) != 1 || len(findings) != 1 || findings[0].Path != "qa/record.toml" || findings[0].Rule != finding.RecordInvalid {
		t.Errorf("got %+v, %v", rec, findings)
	}
}

func TestCheckOfAbsentRecordIsClean(t *testing.T) {
	rec, findings, err := record.Check(project(t, ""), "qa/record.toml")
	if err != nil || len(findings) != 0 || len(rec.Exceptions) != 0 {
		t.Errorf("got %+v, %v, %v", rec, findings, err)
	}
}

func TestCheckPropagatesParseErrorsWithThePath(t *testing.T) {
	_, _, err := record.Check(project(t, "[[exception\n"), "qa/record.toml")
	if err == nil {
		t.Fatal("parse error swallowed")
	}
}
