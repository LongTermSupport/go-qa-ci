package summary_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/builtin"
	"github.com/LongTermSupport/go-qa-ci/internal/defences"
	"github.com/LongTermSupport/go-qa-ci/internal/record"
	"github.com/LongTermSupport/go-qa-ci/internal/summary"
)

func sample() defences.Listing {
	return defences.Listing{
		Defences: []defences.Defence{
			{ID: "errcheck", Engine: defences.EngineGolangci, Level: "error", Description: "Finds unchecked errors."},
			{ID: "shellcheck", Engine: defences.EngineStage, Level: "error", Description: "Lint shell", Fix: "Quote expansions"},
		},
		Stages:  []defences.Step{{Name: "fmt", Phase: "format", Enabled: true}, {Name: "docs", Phase: "detectors"}},
		Runners: []defences.Step{{Name: "test", Phase: "runners", Enabled: true}},
		Record: []record.ListedException{
			{Number: 1, Rule: "errcheck", Path: "a.go", Covers: record.CoversSuppressions, ReviewBy: "2030-01-01", AddedByModel: "example-model"},
		},
		Budget: record.Budget{Total: 1, MaxTotal: 10, MaxPerRule: 3, MaxReviewDays: 180, HighWater: -1},
	}
}

func TestRenderIsDelimitedAndNamesEveryDefenceWithItsPage(t *testing.T) {
	region := summary.Render(sample())
	if !strings.HasPrefix(region, summary.Begin) || !strings.HasSuffix(region, summary.End+"\n") {
		t.Fatalf("region is not delimited:\n%s", region)
	}
	for _, want := range []string{
		"- `errcheck` (golangci, error): Finds unchecked errors. `go-qa rule-doc errcheck`",
		"- `shellcheck` (stage, error): Lint shell. Fix: Quote expansions `go-qa rule-doc shellcheck`",
		"fmt, test",
		"- `errcheck` at `a.go` (review by 2030-01-01, added by example-model)",
		"exceptions: 1 of 10",
	} {
		if !strings.Contains(region, want) {
			t.Errorf("region lacks %q:\n%s", want, region)
		}
	}
	if strings.Contains(region, "docs,") || strings.Contains(region, ", docs") {
		t.Errorf("a disabled step is listed as running:\n%s", region)
	}
}

func TestRenderIsDeterministic(t *testing.T) {
	first, second := summary.Render(sample()), summary.Render(sample())
	if first != second {
		t.Error("two renders differ")
	}
}

func TestSpliceReplacesOnlyTheRegion(t *testing.T) {
	region := summary.Render(sample())
	existing := "# Notes\n\nkeep this\n\n" + summary.Begin + "\nold\n" + summary.End + "\n\nand this\n"
	got, err := summary.Splice(existing, region)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, "# Notes\n\nkeep this\n\n"+summary.Begin) || !strings.HasSuffix(got, summary.End+"\n\nand this\n") || strings.Contains(got, "\nold\n") {
		t.Errorf("got:\n%s", got)
	}
}

func TestSpliceAppendsWhenThereIsNoRegionAndIsIdempotent(t *testing.T) {
	region := summary.Render(sample())
	first, err := summary.Splice("# Notes\n", region)
	if err != nil {
		t.Fatal(err)
	}
	second, err := summary.Splice(first, region)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(first, "# Notes\n\n"+summary.Begin) || first != second {
		t.Errorf("first:\n%s\nsecond:\n%s", first, second)
	}
}

func TestSpliceRefusesBrokenMarkers(t *testing.T) {
	region := summary.Render(sample())
	for name, existing := range map[string]string{
		"begin only":     summary.Begin + "\nx\n",
		"end only":       "x\n" + summary.End + "\n",
		"reversed":       summary.End + "\n" + summary.Begin + "\n",
		"two begin tags": summary.Begin + "\n" + summary.Begin + "\n" + summary.End + "\n",
	} {
		if _, err := summary.Splice(existing, region); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func write(t *testing.T, root, rel, text string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestCheckPassesOnceWrittenAndFailsWhenStale(t *testing.T) {
	root := t.TempDir()
	write(t, root, "AGENTS.md", "# Agents\n")
	f, stale, err := summary.Check(root, "AGENTS.md", sample())
	if err != nil || !stale || f.Rule != builtin.SummaryStale {
		t.Fatalf("an unwritten region: finding %+v, stale %v, err %v", f, stale, err)
	}
	if err = summary.Write(root, "AGENTS.md", sample()); err != nil {
		t.Fatal(err)
	}
	if _, stale, err = summary.Check(root, "AGENTS.md", sample()); err != nil || stale {
		t.Fatalf("a fresh region: stale %v, err %v", stale, err)
	}
	changed := sample()
	changed.Defences = append(changed.Defences, defences.Defence{ID: "extra", Engine: defences.EngineStage, Level: "error"})
	if f, stale, err = summary.Check(root, "AGENTS.md", changed); err != nil || !stale || f.Rule != builtin.SummaryStale || f.Path != "AGENTS.md" {
		t.Fatalf("a stale region: finding %+v, stale %v, err %v", f, stale, err)
	}
}

func TestCheckFailsWhenTheFileIsMissing(t *testing.T) {
	f, stale, err := summary.Check(t.TempDir(), "AGENTS.md", sample())
	if err != nil || !stale || f.Rule != builtin.SummaryStale {
		t.Errorf("finding %+v, stale %v, err %v", f, stale, err)
	}
}

func TestWriteCreatesAMissingFileAndKeepsTheRestOfAnExistingOne(t *testing.T) {
	root := t.TempDir()
	if err := summary.Write(root, "docs/AGENTS.md", sample()); err != nil {
		t.Fatal(err)
	}
	write(t, root, "CLAUDE.md", "keep\n")
	if err := summary.Write(root, "CLAUDE.md", sample()); err != nil {
		t.Fatal(err)
	}
	if data := readFile(t, root, "CLAUDE.md"); !strings.HasPrefix(data, "keep\n\n"+summary.Begin) {
		t.Errorf("file:\n%s", data)
	}
}

// readFile reads a file below root through os.Root, which cannot be steered outside it.
func readFile(t *testing.T, root, rel string) string {
	t.Helper()
	handle, err := os.OpenRoot(root)
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
