package astgrep_test

import (
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/astgrep"
)

const scanJSON = `[{"text":"x","range":{"byteOffset":{"start":0,"end":1},"start":{"line":2,"column":4},"end":{"line":2,"column":5}},
"file":"src/a.go","lines":"x","language":"Go","ruleId":"no-x","severity":"error","message":"no x ","note":" use y\n","metadata":null},
{"text":"z","range":{"byteOffset":{"start":0,"end":1},"start":{"line":0,"column":0},"end":{"line":0,"column":1}},
"file":"src/b.go","lines":"z","language":"Go","ruleId":"hint-z","severity":"hint","message":"z","note":null}]`

func TestParseMatchesGivesOneBasedPositionsAndBlockingErrorsOnly(t *testing.T) {
	t.Parallel()
	matches, err := astgrep.ParseMatches([]byte(scanJSON))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 2 {
		t.Fatalf("got %d matches", len(matches))
	}
	first := matches[0]
	if first.Rule != "no-x" || first.Path != "src/a.go" || first.Line != 3 || first.Column != 5 {
		t.Errorf("first = %+v", first)
	}
	if first.Message != "no x" || first.Note != "use y" || !first.Blocking() {
		t.Errorf("first = %+v", first)
	}
	if matches[1].Blocking() {
		t.Error("only error severity blocks")
	}
}

func TestParseMatchesRejectsTextThatIsNotAMatchList(t *testing.T) {
	t.Parallel()
	for _, text := range []string{"Error: no such path", "", `{"a":1}`, `[{"ruleId":1}]`} {
		if _, err := astgrep.ParseMatches([]byte(text)); err == nil {
			t.Errorf("%q accepted", text)
		}
	}
}

func TestParseMatchesAcceptsAnEmptyList(t *testing.T) {
	t.Parallel()
	matches, err := astgrep.ParseMatches([]byte("[]"))
	if err != nil || len(matches) != 0 {
		t.Errorf("matches=%v err=%v", matches, err)
	}
}
