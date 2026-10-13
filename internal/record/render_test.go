package record_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/record"
)

func sample() record.Record {
	second := entry("govet", "src/b.go", good)
	second.Covers = record.CoversFindings
	return record.Record{
		Ratchet:    &record.Ratchet{HighWater: 2, DecidedBy: "example-owner", Reason: "adopted at two entries"},
		Exceptions: []record.Exception{entry("errcheck", "src/a.go", good), second},
	}
}

func TestListShowsEveryEntryWithItsJustification(t *testing.T) {
	out := sample().List()
	for _, want := range []string{"2 exceptions", "high_water 2", "#1 errcheck", "src/a.go", "#2 govet", "covers findings", good} {
		if !strings.Contains(out, want) {
			t.Errorf("list lacks %q:\n%s", want, out)
		}
	}
}

func TestListOfEmptyRecordSaysSo(t *testing.T) {
	if out := (record.Record{}).List(); !strings.Contains(out, "0 exceptions") {
		t.Errorf("got %q", out)
	}
}

func TestListJSONIsMachineReadable(t *testing.T) {
	data, err := sample().ListJSON()
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Ratchet    map[string]any   `json:"ratchet"`
		Exceptions []map[string]any `json:"exceptions"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.Exceptions) != 2 || decoded.Exceptions[1]["covers"] != "findings" || decoded.Exceptions[0]["rule"] != "errcheck" {
		t.Errorf("got %s", data)
	}
	if decoded.Ratchet["high_water"] != float64(2) {
		t.Errorf("ratchet %v", decoded.Ratchet)
	}
}

func TestExplainByNumberAndByRule(t *testing.T) {
	rec := sample()
	byNumber, err := rec.Explain("2")
	if err != nil || !strings.Contains(byNumber, "govet") || strings.Contains(byNumber, "errcheck") {
		t.Errorf("got %q, %v", byNumber, err)
	}
	byRule, err := rec.Explain("errcheck")
	if err != nil || !strings.Contains(byRule, "src/a.go") || !strings.Contains(byRule, good) {
		t.Errorf("got %q, %v", byRule, err)
	}
}

func TestExplainRejectsUnknownSelectors(t *testing.T) {
	rec := sample()
	for _, sel := range []string{"0", "3", "-1", "nosuchrule", ""} {
		if _, err := rec.Explain(sel); err == nil {
			t.Errorf("Explain(%q) succeeded", sel)
		}
	}
}
