package cli_test

import (
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"
)

type listedDefence struct {
	ID          string `json:"id"`
	Engine      string `json:"engine"`
	Origin      string `json:"origin"`
	Level       string `json:"level"`
	Description string `json:"description"`
	Doc         string `json:"doc"`
}

// rulesJSON is the shape `rules --json` documents.
type rulesJSON struct {
	Defences []listedDefence         `json:"defences"`
	Stages   []struct{ Name string } `json:"stages"`
	Runners  []struct{ Name string } `json:"runners"`
	Record   []json.RawMessage       `json:"record"`
	Budget   struct {
		Total    int `json:"total"`
		MaxTotal int `json:"max_total"`
	} `json:"budget"`
}

func (r rulesJSON) has(id, engine string) bool {
	return slices.ContainsFunc(r.Defences, func(d listedDefence) bool {
		return d.ID == id && d.Engine == engine
	})
}

func decodeRules(t *testing.T, r result) rulesJSON {
	t.Helper()
	if r.code != 0 {
		t.Fatalf("exit %d, stderr %q", r.code, r.err)
	}
	var value rulesJSON
	if err := json.Unmarshal([]byte(r.out), &value); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, r.out)
	}
	return value
}

func TestRulesJSONIsMachineReadable(t *testing.T) {
	value := decodeRules(t, exec("--root", module(t, ""), "rules", "--json"))
	for id, engine := range map[string]string{
		"errcheck":             "golangci",
		"gqaci.record.invalid": "builtin",
		"gqaci_test":           "analyzer",
	} {
		if !value.has(id, engine) {
			t.Errorf("%s (%s) is not listed", id, engine)
		}
	}
	if len(value.Record) != 0 || value.Budget.MaxTotal != 10 || value.Budget.Total != 0 {
		t.Errorf("record %v, budget %+v", value.Record, value.Budget)
	}
	if len(value.Stages) == 0 || value.Stages[0].Name != "fmt" || len(value.Runners) == 0 || value.Runners[0].Name != "vuln" {
		t.Errorf("stages %v, runners %v", value.Stages, value.Runners)
	}
	for _, d := range value.Defences {
		if d.Description == "" || d.Level == "" || d.Origin == "" {
			t.Errorf("%s is listed without a description, level or origin: %+v", d.ID, d)
		}
	}
}

// TestEveryListedDocumentationRouteResolves holds TOOLING-SPEC 5.1: each defence carries a route
// to its full documentation, and a go-qa route is run and must print a page.
func TestEveryListedDocumentationRouteResolves(t *testing.T) {
	root := module(t, "")
	value := decodeRules(t, exec("--root", root, "rules", "--json"))
	const prefix = "go-qa rule-doc "
	for _, d := range value.Defences {
		switch {
		case d.Doc == "golangci-lint help linters" && d.Engine == "golangci":
		case strings.HasPrefix(d.Doc, prefix):
			if id := strings.TrimPrefix(d.Doc, prefix); id != d.ID {
				t.Errorf("%s routes to %q", d.ID, d.Doc)
			}
			if r := exec("--root", root, "rule-doc", d.ID); r.code != 0 {
				t.Errorf("%s: route %q exits %d: %s", d.ID, d.Doc, r.code, r.err)
			}
		default:
			t.Errorf("%s (%s) has no working documentation route: %q", d.ID, d.Engine, d.Doc)
		}
	}
}

func TestRulesShowsARuleAddedToTheConfigurationWithNoOtherEdit(t *testing.T) {
	if decodeRules(t, exec("--root", module(t, ""), "rules", "--json")).has("examples", "stage") {
		t.Fatal("the stage is listed before it is configured")
	}
	root := module(t, stageConfig("examples", "detectors"))
	value := decodeRules(t, exec("--root", root, "rules", "--json"))
	if !value.has("examples", "stage") {
		t.Errorf("the added stage is not listed: %+v", value.Defences)
	}
	text := exec("--root", root, "rules")
	if text.code != 0 || !strings.Contains(text.out, "examples") || !strings.Contains(text.out, "Check the examples") {
		t.Errorf("text listing: %+v", text)
	}
}

func TestRulesListsTheRecordAndItsBudget(t *testing.T) {
	root := module(t, "[tools]\ngolangci = false\n")
	writeFile(t, root, "qa/record.toml", "[[exception]]\nrule = \"errcheck\"\npath = \"p.go\"\njustification = \"j\"\n")
	value := decodeRules(t, exec("--root", root, "rules", "--json"))
	if len(value.Record) != 1 || value.Budget.Total != 1 {
		t.Errorf("record %v, budget %+v", value.Record, value.Budget)
	}
	if value.has("errcheck", "golangci") {
		t.Error("a linter of a disabled tool is listed")
	}
}

func TestRulesTextShowsEachExceptionWithItsJustification(t *testing.T) {
	root := module(t, "[tools]\ngolangci = false\n")
	writeFile(t, root, "qa/record.toml", "[[exception]]\nrule = \"errcheck\"\npath = \"p.go\"\njustification = \"the fixture ignores the error on purpose\"\n")
	r := exec("--root", root, "rules")
	if !strings.Contains(r.out, "#1 errcheck  p.go\n      the fixture ignores the error on purpose\n") {
		t.Errorf("the justification is not beside its exception:\n%s", r.out)
	}
}

const summaryConfig = "[summary]\nfile = \"AGENTS.md\"\n"

func TestSummaryWritesTheRegionAndCheckPassesUntilTheConfigurationChanges(t *testing.T) {
	root := module(t, summaryConfig)
	writeFile(t, root, "AGENTS.md", "# Agents\n\nkeep me\n")
	if r := exec("--root", root, "summary", "--check"); r.code != 1 || !strings.Contains(r.err, "gqaci.summary.stale") {
		t.Fatalf("an unwritten region passed: %+v", r)
	}
	if r := exec("--root", root, "summary"); r.code != 0 {
		t.Fatalf("summary: %+v", r)
	}
	if r := exec("--root", root, "summary", "--check"); r.code != 0 {
		t.Fatalf("a fresh region failed: %+v", r)
	}
	writeFile(t, root, "qa/go-qa.toml", summaryConfig+"\n"+stageConfig("examples", "detectors"))
	r := exec("--root", root, "summary", "--check")
	if r.code != 1 || !strings.Contains(r.err, "gqaci.summary.stale") || !strings.Contains(r.err, "AGENTS.md") {
		t.Fatalf("a stale region passed: %+v", r)
	}
	if r := exec("--root", root, "summary"); r.code != 0 {
		t.Fatalf("summary: %+v", r)
	}
	data := readFile(t, root, "AGENTS.md")
	if !strings.HasPrefix(data, "# Agents\n\nkeep me\n\n") || !strings.Contains(data, "- `examples` (stage, error)") {
		t.Errorf("file:\n%s", data)
	}
}

func TestSummaryNeedsAConfiguredFile(t *testing.T) {
	r := exec("--root", module(t, ""), "summary")
	if r.code != 2 || !strings.Contains(r.err, "summary.file") {
		t.Errorf("got %+v", r)
	}
}

func TestSummaryRejectsUnknownArguments(t *testing.T) {
	r := exec("--root", module(t, summaryConfig), "summary", "--nope")
	if r.code != 2 {
		t.Errorf("got %+v", r)
	}
}

func TestSummaryToolFailsTheGateOnAStaleRegion(t *testing.T) {
	root := module(t, summaryConfig)
	writeFile(t, root, "AGENTS.md", "# Agents\n")
	r := exec("--root", root, "run", "-t", "summary")
	if r.code != 1 || !strings.Contains(r.err, "gqaci.summary.stale") {
		t.Fatalf("a stale region passed the gate: %+v", r)
	}
	if w := exec("--root", root, "summary"); w.code != 0 {
		t.Fatalf("summary: %+v", w)
	}
	if r = exec("--root", root, "run", "-t", "summary"); r.code != 0 {
		t.Errorf("a fresh region failed the gate: %+v", r)
	}
}

func readFile(t *testing.T, dir, rel string) string {
	t.Helper()
	handle, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := handle.Close(); closeErr != nil {
			t.Error(closeErr)
		}
	}()
	data, err := handle.ReadFile(rel)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
