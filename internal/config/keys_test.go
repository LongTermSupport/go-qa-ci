package config

import (
	"reflect"
	"testing"
)

func TestSpellingProblemsNamesEveryMisspeltKey(t *testing.T) {
	raw := map[string]any{
		"Tools":  map[string]any{"FMT": false, "deadcode": true},
		"record": map[string]any{"trust": map[string]any{"models": map[string]any{"Opus": "high"}, "High": map[string]any{"max_count": int64(1)}}},
		"stage":  []any{map[string]any{"Name": "x"}},
	}
	got := spellingProblems(raw, reflect.TypeFor[Config]())
	want := []string{
		`key "Tools" must be spelt "tools"`,
		`key "Tools.FMT" must be spelt "tools.fmt"`,
		`key "record.trust.High" must be spelt "record.trust.high"`,
		`key "stage.Name" must be spelt "stage.name"`,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q\nwant %q", got, want)
	}
}
