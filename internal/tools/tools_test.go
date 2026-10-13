package tools_test

import (
	"reflect"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/config"
	"github.com/LongTermSupport/go-qa-ci/internal/tools"
)

func TestCatalogueIsInPhaseOrder(t *testing.T) {
	want := []string{
		"fmt",
		"record", "suppression", "summary", "docs", "ast-grep", "golangci", "analyzers", "deadcode",
		"vuln", "mod", "test", "coverage", "mutants",
	}
	var got []string
	last := tools.PhaseFormat
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

func TestPhaseNames(t *testing.T) {
	for phase, want := range map[tools.Phase]string{
		tools.PhaseFormat: "format", tools.PhaseDetectors: "detectors", tools.PhaseRunners: "runners",
	} {
		if got := phase.String(); got != want {
			t.Errorf("phase %d = %q, want %q", phase, got, want)
		}
	}
}

func TestFromName(t *testing.T) {
	if tool, ok := tools.FromName("golangci"); !ok || tool.Phase != tools.PhaseDetectors {
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
