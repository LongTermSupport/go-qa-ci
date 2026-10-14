package record_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/date"
	"github.com/LongTermSupport/go-qa-ci/internal/record"
)

const good = "Tests assert on panics deliberately; a panic is the failure signal in test code."

func entry(rule, path, justification string) record.Exception {
	decider := "owner"
	return record.Exception{Rule: rule, Path: path, Justification: justification, DecidedBy: &decider}
}

func write(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "record.toml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func mustDate(t *testing.T, s string) date.Date {
	t.Helper()
	d, err := date.Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestLoadParsesRatchetAndProvenance(t *testing.T) {
	path := write(t, `[ratchet]
high_water = 1
decided_by = "owner"
decided_on = 2026-10-01
reason = "`+good+`"

[[exception]]
rule = "errcheck"
path = "src/a.go"
justification = "`+good+`"
decided_by = "owner"
added_by_model = "human"
decided_on = 2026-10-01
review_by = "2026-12-01"
policy = false
`)
	loaded, err := record.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Ratchet == nil || loaded.Ratchet.HighWater != 1 || !loaded.Ratchet.DecidedOn.Equal(mustDate(t, "2026-10-01")) {
		t.Errorf("ratchet: %+v", loaded.Ratchet)
	}
	e := loaded.Exceptions[0]
	if e.AddedByModel == nil || *e.AddedByModel != "human" || !e.ReviewBy.Equal(mustDate(t, "2026-12-01")) || e.Policy {
		t.Errorf("exception: %+v", e)
	}
	if e.Covers != record.CoversSuppressions {
		t.Errorf("default covers = %q", e.Covers)
	}
	if e.Line != 7 {
		t.Errorf("line = %d, want 7", e.Line)
	}
}

func TestLoadRejectsMalformedDates(t *testing.T) {
	for _, value := range []string{`"next month"`, `2026-10-01T10:00:00Z`, `12`} {
		path := write(t, "[[exception]]\nrule = \"x\"\npath = \"y\"\njustification = \"z\"\nreview_by = "+value+"\n")
		if _, err := record.Load(path); err == nil {
			t.Errorf("review_by = %s accepted", value)
		}
	}
}

func TestLoadMissingFileIsEmptyRecord(t *testing.T) {
	loaded, err := record.Load(filepath.Join(t.TempDir(), "qa", "record.toml"))
	if err != nil || len(loaded.Exceptions) != 0 || loaded.Ratchet != nil {
		t.Errorf("got %+v, %v", loaded, err)
	}
}

func TestLoadParsesExceptions(t *testing.T) {
	path := write(t, "[[exception]]\nrule = \"errcheck\"\npath = \"tests/a.go\"\ncovers = \"findings\"\njustification = \""+good+"\"\n")
	loaded, err := record.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	e := loaded.Exceptions[0]
	if len(loaded.Exceptions) != 1 || e.Rule != "errcheck" || e.DecidedBy != nil || e.Covers != record.CoversFindings {
		t.Errorf("got %+v", loaded.Exceptions)
	}
}

func TestLoadRejectsUnknownKeysAndCovers(t *testing.T) {
	cases := map[string]string{
		"unknown key":   "[[exception]]\nrule = \"x\"\npath = \"y\"\njustification = \"z\"\nreson = \"typo\"\n",
		"unknown table": "[ratchets]\nhigh_water = 1\n",
		"bad covers":    "[[exception]]\nrule = \"x\"\npath = \"y\"\njustification = \"z\"\ncovers = \"everything\"\n",
		"bad toml":      "[[exception\n",
	}
	for name, content := range cases {
		if _, err := record.Load(write(t, content)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestLoadNamesUnknownKey(t *testing.T) {
	_, err := record.Load(write(t, "[[exception]]\nreson = \"typo\"\n"))
	if err == nil || !strings.Contains(err.Error(), "reson") {
		t.Errorf("error %v does not name the key", err)
	}
}

func TestEntryLinesSurviveHeadersWithCommentsSpacesAndStringLookalikes(t *testing.T) {
	text := "[[exception]] # first\n" +
		"rule = \"a\"\n" +
		"path = \"a.go\"\n" +
		"justification = \"\"\"\n[[exception]]\nnot a header, only text\n\"\"\"\n" +
		"[[ exception ]]\n" +
		"rule = \"b\"\n" +
		"path = \"b.go\"\n" +
		"[[exception]]   # third\n" +
		"rule = \"c\"\n" +
		"path = \"c.go\"\n"
	loaded, err := record.Parse([]byte(text))
	if err != nil {
		t.Fatal(err)
	}
	var got []int
	for _, e := range loaded.Exceptions {
		got = append(got, e.Line)
	}
	if want := []int{1, 8, 11}; !slices.Equal(got, want) {
		t.Errorf("entry lines %v, want %v", got, want)
	}
}
