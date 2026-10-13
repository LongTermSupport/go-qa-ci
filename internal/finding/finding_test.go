package finding_test

import (
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/finding"
)

func TestStringIsCompilerStyleWithIdentifier(t *testing.T) {
	f := finding.Finding{Rule: finding.RecordInvalid, Path: "qa/record.toml", Line: 7, Message: "bad entry"}
	want := "error[gqaci.record.invalid]: bad entry\n  --> qa/record.toml:7"
	if got := f.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestStringOmitsLineWhenUnknown(t *testing.T) {
	f := finding.Finding{Rule: finding.RecordStale, Path: "qa/record.toml", Message: "unused"}
	want := "error[gqaci.record.stale]: unused\n  --> qa/record.toml"
	if got := f.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
