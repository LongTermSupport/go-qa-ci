package tools_test

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/config"
	"github.com/LongTermSupport/go-qa-ci/internal/pipeline"
	"github.com/LongTermSupport/go-qa-ci/internal/tools"
)

func TestCatalogueIsInPhaseOrder(t *testing.T) {
	want := []string{
		"fmt",
		"record", "suppression", "summary", "docs", "ast-grep", "golangci", "analyzers", "deadcode",
		"vuln", "mod", "test", "coverage", "mutants",
	}
	var got []string
	last := pipeline.PhaseFormat
	for _, tool := range tools.Catalogue() {
		got = append(got, tool.Name)
		if tool.Phase < last {
			t.Errorf("%s (%s) comes after a later phase", tool.Name, tool.Phase)
		}
		last = tool.Phase
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("catalogue = %v, want %v", got, want)
	}
}

func TestFromName(t *testing.T) {
	if tool, ok := tools.FromName("golangci"); !ok || tool.Phase != pipeline.PhaseDetectors {
		t.Errorf("golangci = %+v, %v", tool, ok)
	}
	if _, ok := tools.FromName("clippy"); ok {
		t.Error("clippy is not a go-qa tool")
	}
}

func TestEnabledFollowsConfig(t *testing.T) {
	cfg := config.Default()
	enabled := map[string]bool{}
	for _, tool := range tools.Catalogue() {
		enabled[tool.Name] = tool.Enabled(cfg.Tools)
	}
	for name, want := range map[string]bool{"fmt": true, "vuln": true, "mutants": true, "deadcode": true} {
		if enabled[name] != want {
			t.Errorf("default %s = %v, want %v", name, enabled[name], want)
		}
	}
	cfg.Tools = config.Tools{}
	for _, tool := range tools.Catalogue() {
		if tool.Enabled(cfg.Tools) {
			t.Errorf("%s enabled with every switch off", tool.Name)
		}
	}
}

func TestEverySwitchControlsExactlyOneTool(t *testing.T) {
	all := config.Tools{
		Fmt: true, Record: true, Suppression: true, Summary: true, Docs: true, AstGrep: true,
		Golangci: true, Analyzers: true, Deadcode: true, Vuln: true, Mod: true, Test: true,
		Coverage: true, Mutants: true,
	}
	for _, tool := range tools.Catalogue() {
		if !tool.Enabled(all) {
			t.Errorf("%s is not controlled by a switch", tool.Name)
		}
	}
}

func TestSelectWithoutNamesIsTheEnabledToolsInCatalogueOrder(t *testing.T) {
	cfg := config.Default()
	cfg.Tools.Mutants = false
	cfg.Tools.Fmt = false
	steps, err := tools.Select(nil, cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range steps {
		if step.Name == "mutants" || step.Name == "fmt" {
			t.Errorf("disabled tool %s selected", step.Name)
		}
	}
	if len(steps) == 0 || steps[0].Name != "record" {
		t.Errorf("steps = %v", steps)
	}
}

func TestSelectNamedToolsIgnoresTheSwitchAndKeepsCatalogueRank(t *testing.T) {
	steps, err := tools.Select([]string{"test", "fmt", "test"}, config.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 3 || steps[0].Name != "test" || steps[1].Name != "fmt" || steps[0].Rank <= steps[1].Rank {
		t.Errorf("steps = %v", steps)
	}
	if steps[0].Phase != pipeline.PhaseRunners {
		t.Errorf("test phase = %v", steps[0].Phase)
	}
}

func TestSelectUnknownNameListsTheKnownOnes(t *testing.T) {
	_, err := tools.Select([]string{"clippy"}, config.Default())
	if err == nil {
		t.Fatal("no error")
	}
	for _, want := range append([]string{`"clippy"`}, tools.Names()...) {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q lacks %q", err, want)
		}
	}
}

func stepNames(steps []pipeline.Step) []string {
	names := make([]string, len(steps))
	for i, step := range steps {
		names[i] = step.Name
	}
	return names
}

func TestSelectNamingADetectorStillRunsTheRecordCheckFirst(t *testing.T) {
	steps, err := tools.Select([]string{"suppression"}, config.Default())
	if err != nil {
		t.Fatal(err)
	}
	got := stepNames(steps)
	if len(got) != 2 || !slices.Contains(got, "record") || !slices.Contains(got, "suppression") {
		t.Errorf("steps = %v", got)
	}
	slices.SortFunc(steps, func(a, b pipeline.Step) int { return a.Rank - b.Rank })
	if steps[0].Name != "record" {
		t.Errorf("record does not rank before suppression: %v", steps)
	}
}

func TestSelectDoesNotDuplicateARecordCheckThatWasNamed(t *testing.T) {
	steps, err := tools.Select([]string{"record", "suppression"}, config.Default())
	if err != nil {
		t.Fatal(err)
	}
	if got := stepNames(steps); len(got) != 2 {
		t.Errorf("steps = %v", got)
	}
}

func TestSelectLeavesFormatAloneAndHonoursTheRecordSwitch(t *testing.T) {
	steps, err := tools.Select([]string{"fmt"}, config.Default())
	if err != nil {
		t.Fatal(err)
	}
	if got := stepNames(steps); len(got) != 1 {
		t.Errorf("fmt alone selected %v", got)
	}
	cfg := config.Default()
	cfg.Tools.Record = false
	steps, err = tools.Select([]string{"suppression"}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if got := stepNames(steps); len(got) != 1 {
		t.Errorf("record switched off but selected %v", got)
	}
}
