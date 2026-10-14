package defences_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/builtin"
	"github.com/LongTermSupport/go-qa-ci/internal/config"
	"github.com/LongTermSupport/go-qa-ci/internal/defences"
	"github.com/LongTermSupport/go-qa-ci/internal/stage"
	"github.com/LongTermSupport/go-qa-ci/internal/vettest"
)

// The glob is assembled from parts so that no source line holds a comment opener.
const anyShell = "**/" + "*.sh"

// TestMain lets this test binary stand in for go-qa as the go vet tool, as the binary does in
// production when the analyzers are listed.
func TestMain(m *testing.M) {
	vettest.Serve()
	os.Exit(m.Run())
}

func module(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	files["go.mod"] = "module example.com/fixture\n\ngo 1.26\n"
	for name, text := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// allSteps are the catalogue as the executor would hand it over: every tool enabled.
func allSteps(cfg config.Config) []defences.Step {
	steps := []defences.Step{
		{Name: "fmt", Phase: "format", Enabled: true},
		{Name: "record", Phase: "detectors", Enabled: true},
		{Name: "suppression", Phase: "detectors", Enabled: true},
		{Name: "summary", Phase: "detectors", Enabled: true},
		{Name: "docs", Phase: "detectors", Enabled: true},
		{Name: "ast-grep", Phase: "detectors", Enabled: true},
		{Name: "golangci", Phase: "detectors", Enabled: true},
		{Name: "analyzers", Phase: "detectors", Enabled: true},
		{Name: "test", Phase: "runners", Enabled: true},
	}
	for _, s := range cfg.Stages {
		steps = append(steps, defences.Step{Name: s.Name, Phase: s.Phase, Project: true, Enabled: true})
	}
	return steps
}

func collect(t *testing.T, root string, cfg config.Config, steps []defences.Step) defences.Listing {
	t.Helper()
	listing, err := defences.Collect(context.Background(), defences.Input{
		Root: root, Config: cfg, Steps: steps,
	})
	if err != nil {
		t.Fatal(err)
	}
	return listing
}

func find(listing defences.Listing, id string) (defences.Defence, bool) {
	i := slices.IndexFunc(listing.Defences, func(d defences.Defence) bool { return d.ID == id })
	if i < 0 {
		return defences.Defence{}, false
	}
	return listing.Defences[i], true
}

func TestCollectListsEveryEngineWithItsDescription(t *testing.T) {
	cfg := config.Default()
	listing := collect(t, module(t, map[string]string{"p.go": "package p\n"}), cfg, allSteps(cfg))
	for id, engine := range map[string]string{
		builtin.RecordInvalid: defences.EngineBuiltin,
		"errcheck":            defences.EngineGolangci,
		"gqaci_test":          defences.EngineAnalyzer,
	} {
		d, found := find(listing, id)
		if !found || d.Engine != engine || d.Description == "" || d.Level != "error" {
			t.Errorf("%s: got %+v (found %v), want engine %s with a description", id, d, found, engine)
		}
	}
}

func TestCollectFillsTheFixFromTheRulePage(t *testing.T) {
	cfg := config.Default()
	listing := collect(t, module(t, map[string]string{"p.go": "package p\n"}), cfg, allSteps(cfg))
	d, found := find(listing, builtin.RecordInvalid)
	if !found || d.Fix == "" {
		t.Errorf("the bundled page's fix is not quoted: %+v (found %v)", d, found)
	}
}

func TestCollectCoversEveryBuiltinOwnedByAnEnabledTool(t *testing.T) {
	cfg := config.Default()
	listing := collect(t, module(t, map[string]string{"p.go": "package p\n"}), cfg, allSteps(cfg))
	for _, id := range builtin.All() {
		if id == builtin.SensitiveFormat {
			continue // an analyzer: listed with the analyzers of the vet tool
		}
		if _, found := find(listing, id); !found {
			t.Errorf("built-in %s is not listed", id)
		}
	}
}

func TestCollectOmitsTheDefencesOfADisabledTool(t *testing.T) {
	cfg := config.Default()
	steps := allSteps(cfg)
	for i := range steps {
		if steps[i].Name == "golangci" || steps[i].Name == "record" || steps[i].Name == "analyzers" {
			steps[i].Enabled = false
		}
	}
	listing := collect(t, module(t, map[string]string{"p.go": "package p\n"}), cfg, steps)
	for _, id := range []string{"errcheck", builtin.RecordInvalid, "gqaci_test"} {
		if _, found := find(listing, id); found {
			t.Errorf("%s is listed although its tool is off", id)
		}
	}
	if _, found := find(listing, builtin.DocsDangling); !found {
		t.Error("a defence of an enabled tool disappeared")
	}
}

func TestCollectListsAProjectStageWithNoOtherEdit(t *testing.T) {
	cfg := config.Default()
	before := collect(t, module(t, map[string]string{"p.go": "package p\n"}), cfg, allSteps(cfg))
	if _, found := find(before, "shellcheck"); found {
		t.Fatal("the stage is listed before it is configured")
	}
	cfg.Stages = []stage.Stage{{
		Name: "shellcheck", Phase: "detectors", Says: "ShellCheck over every shell script",
		Fix: "Quote expansions", Command: []string{"shellcheck", stage.Files}, Files: []string{anyShell},
	}}
	after := collect(t, module(t, map[string]string{"p.go": "package p\n"}), cfg, allSteps(cfg))
	d, found := find(after, "shellcheck")
	if !found || d.Engine != defences.EngineStage || d.Origin != defences.OriginProject || d.Fix != "Quote expansions" || d.Description != "ShellCheck over every shell script" {
		t.Errorf("got %+v (found %v)", d, found)
	}
}

func TestCollectListsProjectAstGrepRulesBesideTheBundledOnes(t *testing.T) {
	cfg := config.Default()
	root := module(t, map[string]string{
		"p.go":              "package p\n",
		"sgconfig.yml":      "ruleDirs: [rules]\n",
		"rules/no-todo.yml": "id: no-todo\nlanguage: go\nseverity: error\nmessage: no todo markers\nrule:\n  pattern: x\n",
	})
	d, found := find(collect(t, root, cfg, allSteps(cfg)), "no-todo")
	if !found || d.Engine != defences.EngineAstGrep || d.Origin != defences.OriginProject || d.Level != "error" || d.Description != "no todo markers" {
		t.Errorf("got %+v (found %v)", d, found)
	}
}

func TestCollectSplitsTheStepsAndKeepsTheRecordWithItsBudget(t *testing.T) {
	cfg := config.Default()
	root := module(t, map[string]string{"p.go": "package p\n"})
	listing := collect(t, root, cfg, allSteps(cfg))
	if len(listing.Runners) != 1 || listing.Runners[0].Name != "test" {
		t.Errorf("runners = %+v", listing.Runners)
	}
	if len(listing.Stages) != 8 || listing.Stages[0].Name != "fmt" {
		t.Errorf("stages = %+v", listing.Stages)
	}
	if len(listing.Record) != 0 || listing.Budget.MaxTotal != cfg.Record.MaxTotal || listing.Budget.Total != 0 {
		t.Errorf("record = %+v, budget = %+v", listing.Record, listing.Budget)
	}
}

func TestCollectReportsTheRecordItReads(t *testing.T) {
	cfg := config.Default()
	root := module(t, map[string]string{
		"p.go":           "package p\n",
		"qa/record.toml": "[[exception]]\nrule = \"errcheck\"\npath = \"p.go\"\njustification = \"j\"\n",
	})
	listing := collect(t, root, cfg, allSteps(cfg))
	if len(listing.Record) != 1 || listing.Record[0].Rule != "errcheck" || listing.Budget.Total != 1 {
		t.Errorf("record = %+v, budget = %+v", listing.Record, listing.Budget)
	}
}

func TestListingJSONHasTheDocumentedShape(t *testing.T) {
	cfg := config.Default()
	listing := collect(t, module(t, map[string]string{"p.go": "package p\n"}), cfg, allSteps(cfg))
	data, err := json.Marshal(listing)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"defences", "stages", "runners", "record", "budget"} {
		if _, present := decoded[key]; !present {
			t.Errorf("key %q missing from %s", key, data)
		}
	}
}

func TestCollectFailsOnABuiltinNoToolOwns(t *testing.T) {
	cfg := config.Default()
	steps := slices.DeleteFunc(allSteps(cfg), func(s defences.Step) bool { return s.Name == "docs" })
	_, err := defences.Collect(context.Background(), defences.Input{
		Root: module(t, map[string]string{"p.go": "package p\n"}), Config: cfg, Steps: steps,
	})
	if err == nil {
		t.Error("a catalogue without the docs tool was accepted")
	}
}
