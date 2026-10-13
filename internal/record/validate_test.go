package record_test

import (
	"strings"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/finding"
	"github.com/LongTermSupport/go-qa-ci/internal/record"
)

func TestSpecificJustificationIsAccepted(t *testing.T) {
	if problem := record.JustificationProblem(good); problem != "" {
		t.Errorf("rejected: %s", problem)
	}
	rec := record.Record{Exceptions: []record.Exception{entry("errcheck", "src/a.go", good)}}
	if findings := rec.Validate("qa/record.toml"); len(findings) != 0 {
		t.Errorf("%v", findings)
	}
}

func TestEmptyJustificationIsRejected(t *testing.T) {
	if record.JustificationProblem("   ") == "" {
		t.Error("blank accepted")
	}
}

func TestGenericJustificationsAreRejected(t *testing.T) {
	for _, text := range []string{"TODO", "legacy", "Needed for now.", "temporary", "n/a", "fix later", "WIP", "false positive", "  Legacy code!  "} {
		if record.JustificationProblem(text) == "" {
			t.Errorf("%q accepted", text)
		}
	}
}

func TestPlaceholderWordsInsideLongerTextAreRejected(t *testing.T) {
	for _, text := range []string{
		"TODO: explain properly why this lint is wrong here",
		"FIXME once the upstream package exposes a typed error",
		"tbd after the migration of the whole test suite",
	} {
		if record.JustificationProblem(text) == "" {
			t.Errorf("%q accepted", text)
		}
	}
}

func TestTooShortJustificationIsRejected(t *testing.T) {
	if record.JustificationProblem("it is fine") == "" {
		t.Error("short text accepted")
	}
	if record.JustificationProblem(strings.Repeat("a", record.MinJustificationChars-1)) == "" {
		t.Error("one below the minimum accepted")
	}
	if p := record.JustificationProblem(strings.Repeat("a", record.MinJustificationChars)); p != "" {
		t.Errorf("exactly the minimum rejected: %s", p)
	}
}

func TestLengthCountsCharactersNotBytes(t *testing.T) {
	text := strings.Repeat("é", record.MinJustificationChars)
	if p := record.JustificationProblem(text); p != "" {
		t.Errorf("rejected: %s", p)
	}
}

func TestValidateReportsEachBadExceptionWithItsRule(t *testing.T) {
	rec := record.Record{Exceptions: []record.Exception{
		entry("errcheck", "src/a.go", "legacy"),
		entry("", "src/a.go", good),
		entry("errcheck", "", good),
		entry("errcheck", "src/[a.go", good),
		entry("errcheck", "src/a.go", good),
	}}
	findings := rec.Validate("qa/record.toml")
	if len(findings) != 4 {
		t.Fatalf("got %d findings: %v", len(findings), findings)
	}
	for _, f := range findings {
		if f.Rule != finding.RecordInvalid || f.Path != "qa/record.toml" {
			t.Errorf("finding %+v", f)
		}
	}
	if !strings.Contains(findings[0].Message, "errcheck") || !strings.Contains(findings[0].Message, "#1") {
		t.Errorf("message %q", findings[0].Message)
	}
}

func TestGlobWithoutPolicyIsRejected(t *testing.T) {
	plain := entry("errcheck", "**/a.go", good)
	findings := record.Record{Exceptions: []record.Exception{plain}}.Validate("r.toml")
	if len(findings) != 1 || !strings.Contains(findings[0].Message, "policy = true") {
		t.Fatalf("got %v", findings)
	}
	broad := plain
	broad.Policy = true
	if findings := (record.Record{Exceptions: []record.Exception{broad}}).Validate("r.toml"); len(findings) != 0 {
		t.Errorf("policy glob rejected: %v", findings)
	}
}

func TestEmptyDeciderIsRejected(t *testing.T) {
	blank := " "
	bad := entry("errcheck", "src/a.go", good)
	bad.DecidedBy = &blank
	if findings := (record.Record{Exceptions: []record.Exception{bad}}).Validate("r.toml"); len(findings) != 1 {
		t.Errorf("got %v", findings)
	}
}

func TestAProblemListIsOneFindingPerException(t *testing.T) {
	findings := record.Record{Exceptions: []record.Exception{entry("", "", "")}}.Validate("r.toml")
	if len(findings) != 1 {
		t.Fatalf("got %d", len(findings))
	}
	for _, want := range []string{"no rule", "no path", "justification is empty"} {
		if !strings.Contains(findings[0].Message, want) {
			t.Errorf("%q lacks %q", findings[0].Message, want)
		}
	}
}

func TestFindingCarriesTheEntryLine(t *testing.T) {
	path := write(t, "[[exception]]\nrule = \"a\"\npath = \"b\"\njustification = \"legacy\"\n\n[[exception]]\nrule = \"c\"\npath = \"d\"\njustification = \"todo\"\n")
	loaded, err := record.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	findings := loaded.Validate("qa/record.toml")
	if len(findings) != 2 || findings[0].Line != 1 || findings[1].Line != 6 {
		t.Errorf("got %v", findings)
	}
}
