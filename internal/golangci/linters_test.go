package golangci_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/golangci"
)

func TestParseLintersKeepsTheEnabledOnesSortedByName(t *testing.T) {
	t.Parallel()
	data := `{"Enabled":[{"name":"vet","description":"Reports suspicious constructs."},{"name":"errcheck","description":"Finds unchecked errors.","fast":false}],
"Disabled":[{"name":"godot","description":"Checks comments."}]}`
	got, err := golangci.ParseLinters([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	want := []golangci.Linter{
		{Name: "errcheck", Description: "Finds unchecked errors."},
		{Name: "vet", Description: "Reports suspicious constructs."},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestParseLintersRejectsOutputWithoutAnEnabledList(t *testing.T) {
	t.Parallel()
	for _, data := range []string{"", "not json", `{"Disabled":[]}`} {
		if _, err := golangci.ParseLinters([]byte(data)); err == nil {
			t.Errorf("accepted %q", data)
		}
	}
}

func TestLintersListsWhatTheConfigurationEnables(t *testing.T) {
	runner := locate(t)
	dir := module(t, map[string]string{"p.go": "package p\n"})
	configPath, cleanup, err := golangci.Config(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	linters, err := runner.Linters(context.Background(), dir, configPath)
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]string{}
	for _, l := range linters {
		found[l.Name] = l.Description
	}
	if found["errcheck"] == "" {
		t.Errorf("errcheck missing or undescribed: %v", found)
	}
	if _, listed := found["arangolint"]; listed {
		t.Errorf("a linter the baseline leaves off is listed: %v", found)
	}
}
