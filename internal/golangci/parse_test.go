package golangci_test

import (
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/finding"
	"github.com/LongTermSupport/go-qa-ci/internal/golangci"
)

const sample = `{"Issues":[
 {"FromLinter":"errcheck","Text":"Error return value of ` + "`os.Remove`" + ` is not checked","Pos":{"Filename":"main.go","Line":9,"Column":11}},
 {"FromLinter":"staticcheck","Text":"SA1019: x.Old is deprecated: use New","Pos":{"Filename":"pkg/b.go","Line":3,"Column":1}},
 {"FromLinter":"gosec","Text":"G402: TLS InsecureSkipVerify set to true.","Pos":{"Filename":"c.go","Line":4,"Column":1}},
 {"FromLinter":"govet","Text":"printf: fmt.Printf format %d has arg \"x\" of wrong type string","Pos":{"Filename":"c.go","Line":10,"Column":2}},
 {"FromLinter":"unused","Text":"func use is unused: not called","Pos":{"Filename":"b.go","Line":5,"Column":6}},
 {"FromLinter":"staticcheck","Text":"no code here","Pos":{"Filename":"b.go","Line":6,"Column":6}}
],"Report":{"Linters":[{"Name":"errcheck"}]}}`

func TestParseKeepsIdentifiersUnaltered(t *testing.T) {
	got, err := golangci.Parse([]byte(sample))
	if err != nil {
		t.Fatal(err)
	}
	want := []finding.Finding{
		{Rule: "errcheck", Path: "main.go", Line: 9, Message: "Error return value of `os.Remove` is not checked"},
		{Rule: "staticcheck::SA1019", Path: "pkg/b.go", Line: 3, Message: "x.Old is deprecated: use New"},
		{Rule: "gosec::G402", Path: "c.go", Line: 4, Message: "TLS InsecureSkipVerify set to true."},
		{Rule: "govet::printf", Path: "c.go", Line: 10, Message: "fmt.Printf format %d has arg \"x\" of wrong type string"},
		{Rule: "unused", Path: "b.go", Line: 5, Message: "func use is unused: not called"},
		{Rule: "staticcheck", Path: "b.go", Line: 6, Message: "no code here"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d findings: %+v", len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("finding %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestParseRendersCompilerStyle(t *testing.T) {
	got, err := golangci.Parse([]byte(sample))
	if err != nil {
		t.Fatal(err)
	}
	if text := got[0].String(); text != "error[errcheck]: Error return value of `os.Remove` is not checked\n  --> main.go:9" {
		t.Errorf("got %q", text)
	}
}

func TestParseRejectsMalformedOutput(t *testing.T) {
	for _, in := range []string{"", "not json", `{"Issues":"x"}`, `{"Issues":[{"FromLinter":"","Text":"t","Pos":{"Filename":"a.go"}}]}`} {
		if _, err := golangci.Parse([]byte(in)); err == nil {
			t.Errorf("%q parsed without error", in)
		}
	}
}

func TestParseWithoutIssues(t *testing.T) {
	for _, in := range []string{`{"Issues":[],"Report":{}}`, `{"Issues":null,"Report":{}}`} {
		got, err := golangci.Parse([]byte(in))
		if err != nil || len(got) != 0 {
			t.Errorf("%s: got %+v, %v", in, got, err)
		}
	}
}

func TestCodePrefixedLintersAreSortedAndNameTheFiveWrappers(t *testing.T) {
	got := golangci.CodePrefixedLinters()
	want := []string{"gocritic", "gosec", "govet", "revive", "staticcheck"}
	if len(got) != len(want) {
		t.Fatalf("code-prefixed linters = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("code-prefixed linters = %v, want %v", got, want)
		}
	}
}
