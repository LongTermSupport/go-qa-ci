package sensitiveformat_test

import (
	"slices"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/LongTermSupport/go-qa-ci/analyzers/sensitiveformat"
)

func TestRedFixtures(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), sensitiveformat.Analyzer, "red")
}

func TestCleanFixture(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), sensitiveformat.Analyzer, "clean")
}

func TestConfiguredPatternsAndRedactingTypes(t *testing.T) {
	analyzer := sensitiveformat.New(sensitiveformat.Config{
		Patterns:  []string{"hush", "opaque"},
		Redacting: []string{"configured.Opaque"},
	})
	analysistest.Run(t, analysistest.TestData(), analyzer, "configured")
}

func TestIdentifierAndDocPage(t *testing.T) {
	if sensitiveformat.Analyzer.Name != "gqaci_sensitive_format" {
		t.Errorf("name %q", sensitiveformat.Analyzer.Name)
	}
	if sensitiveformat.Analyzer.Doc == "" {
		t.Error("an analyzer Doc is the fall-back page and must not be empty")
	}
}

func TestFlagsCarryTheConfiguration(t *testing.T) {
	analyzer := sensitiveformat.New(sensitiveformat.Config{Patterns: []string{"a"}})
	for _, name := range []string{"patterns", "redacting"} {
		if analyzer.Flags.Lookup(name) == nil {
			t.Errorf("flag %q is not registered", name)
		}
	}
	if err := analyzer.Flags.Set("patterns", "x,y"); err != nil {
		t.Fatal(err)
	}
	if got := analyzer.Flags.Lookup("patterns").Value.String(); got != "x,y" {
		t.Errorf("patterns = %q", got)
	}
	if err := analyzer.Flags.Set("patterns", ""); err == nil {
		t.Error("an empty pattern list would switch the rule off and must be rejected")
	}
}

func TestDefaultPatternsAreNotEmpty(t *testing.T) {
	if len(sensitiveformat.DefaultConfig().Patterns) == 0 {
		t.Fatal("no default patterns")
	}
	if !slices.Contains(sensitiveformat.DefaultConfig().Patterns, "password") {
		t.Error("password is not a default pattern")
	}
}

func TestNewRefusesAnEmptyPatternListBecauseItSwitchesTheRuleOff(t *testing.T) {
	for _, cfg := range []sensitiveformat.Config{{}, {Patterns: []string{" ", ""}}} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("New(%+v) did not panic", cfg)
				}
			}()
			sensitiveformat.New(cfg)
		}()
	}
}

func TestBuiltRecognisesOnlyAnalyzersMadeByNew(t *testing.T) {
	if !sensitiveformat.Built(sensitiveformat.Analyzer) || !sensitiveformat.Built(sensitiveformat.New(sensitiveformat.Config{Patterns: []string{"a"}})) {
		t.Error("an analyzer made by New is not recognised")
	}
	if sensitiveformat.Built(&analysis.Analyzer{Name: sensitiveformat.Name}) || sensitiveformat.Built(nil) {
		t.Error("an impostor is recognised")
	}
}
