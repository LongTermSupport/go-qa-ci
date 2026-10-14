package stage_test

import (
	"reflect"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/stage"
)

func TestFindingsPrintAsJSONRelativeToTheStageDirectory(t *testing.T) {
	json := `[{"file":"src/a.ts","line":4,"rule":"web-rule","message":"m","note":"n"},
	          {"file":"b.yml","line":1,"column":3,"rule":"yaml-rule","message":"m2"}]`
	found, err := stage.ParseFindings(json, "web")
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 2 || found[0].Path != "web/src/a.ts" || found[0].Line != 4 || found[0].Rule != "web-rule" {
		t.Fatalf("findings = %+v", found)
	}
	if found[0].Message != "m (n)" || found[1].Message != "m2" || found[1].Path != "web/b.yml" {
		t.Errorf("findings = %+v", found)
	}
	if again, parseErr := stage.ParseFindings(`[]`, ""); parseErr != nil || len(again) != 0 {
		t.Errorf("an empty list: %v, %v", again, parseErr)
	}
}

func TestFindingsThatAreNotTheContractAreRejected(t *testing.T) {
	for name, json := range map[string]string{
		"not json":        "not json",
		"not a list":      `{"file":"a"}`,
		"unknown field":   `[{"file":"a","line":1,"rule":"r","message":"m","severity":"x"}]`,
		"no rule":         `[{"file":"a","line":1,"message":"m"}]`,
		"no file":         `[{"line":1,"rule":"r","message":"m"}]`,
		"no message":      `[{"file":"a","line":1,"rule":"r"}]`,
		"negative line":   `[{"file":"a","line":-1,"rule":"r","message":"m"}]`,
		"escaping path":   `[{"file":"../a","line":1,"rule":"r","message":"m"}]`,
		"absolute path":   `[{"file":"/etc/a","line":1,"rule":"r","message":"m"}]`,
		"trailing text":   `[] extra`,
		"wrong line type": `[{"file":"a","line":"1","rule":"r","message":"m"}]`,
	} {
		if _, err := stage.ParseFindings(json, ""); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestRulesPrintAsJSONWithADescriptionEach(t *testing.T) {
	rules, err := stage.ParseRules(`[{"id":"web-rule","description":"what it finds","doc":"# web-rule"}]`)
	want := []stage.Rule{{ID: "web-rule", Description: "what it finds", Doc: "# web-rule"}}
	if err != nil || !reflect.DeepEqual(rules, want) {
		t.Errorf("rules = %+v, %v", rules, err)
	}
	for _, bad := range []string{`[{"id": 3}]`, `[{"description":"x"}]`, `[{"id":"a"},{"id":"a"}]`, `nope`, `[{"id":"a","extra":1}]`} {
		if _, err := stage.ParseRules(bad); err == nil {
			t.Errorf("accepted %s", bad)
		}
	}
}
