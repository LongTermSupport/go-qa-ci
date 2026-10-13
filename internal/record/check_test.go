package record_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/config"

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
	rec, findings, err := record.Check(root, checkPolicy(), today(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(rec.Exceptions) != 1 || len(findings) != 3 || findings[0].Path != "qa/record.toml" || findings[0].Rule != finding.RecordInvalid {
		t.Errorf("got %+v, %v", rec, findings)
	}
}

func TestCheckOfAbsentRecordIsClean(t *testing.T) {
	rec, findings, err := record.Check(project(t, ""), checkPolicy(), today(t))
	if err != nil || len(findings) != 0 || len(rec.Exceptions) != 0 {
		t.Errorf("got %+v, %v, %v", rec, findings, err)
	}
}

func TestCheckPropagatesParseErrorsWithThePath(t *testing.T) {
	_, _, err := record.Check(project(t, "[[exception\n"), checkPolicy(), today(t))
	if err == nil {
		t.Fatal("parse error swallowed")
	}
}

func checkPolicy() config.RecordPolicy { return config.Default().Record }

func TestCheckAppliesThePolicyReviewWithTheGivenClock(t *testing.T) {
	root := project(t, "[ratchet]\nhigh_water = 1\ndecided_by = \"alice\"\ndecided_on = 2026-06-01\nreason = \""+soundReason+"\"\n\n"+
		"[[exception]]\nrule = \"errcheck\"\npath = \"a.go\"\njustification = \""+soundReason+"\"\n"+
		"decided_by = \"alice\"\nadded_by_model = \"human\"\ndecided_on = 2026-06-01\nreview_by = 2026-07-01\n")
	policy := checkPolicy()
	policy.Deciders = []string{"alice"}
	_, findings, err := record.Check(root, policy, today(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 || findings[0].Rule != finding.RecordExpired || findings[0].Line != 7 {
		t.Errorf("got %v", findings)
	}
}
