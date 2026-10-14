package astgrep_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/LongTermSupport/go-qa-ci/internal/astgrep"
)

const goodRule = "id: gqaci.ast.example\nlanguage: go\nseverity: error\nmessage: m\nnote: \"go-qa rule-doc gqaci.ast.example\"\nrule:\n  pattern: x\n"

func TestLoadPackReadsEveryRuleFileInOrder(t *testing.T) {
	t.Parallel()
	files := fstest.MapFS{
		"gqaci.ast.b.yml": {Data: []byte(strings.ReplaceAll(goodRule, "example", "b"))},
		"gqaci.ast.a.yml": {Data: []byte(strings.ReplaceAll(goodRule, "example", "a"))},
		"README":          {Data: []byte("not a rule")},
	}
	pack, err := astgrep.LoadPack(files)
	if err != nil {
		t.Fatal(err)
	}
	if ids := pack.IDs(); len(ids) != 2 || ids[0] != "gqaci.ast.a" || ids[1] != "gqaci.ast.b" {
		t.Errorf("ids = %v", ids)
	}
}

func TestLoadPackRejectsAnInvalidRule(t *testing.T) {
	t.Parallel()
	for name, text := range map[string]string{
		"not namespaced":       strings.ReplaceAll(goodRule, "gqaci.ast.example", "plain"),
		"not blocking":         strings.ReplaceAll(goodRule, "severity: error", "severity: warning"),
		"not go":               strings.ReplaceAll(goodRule, "language: go", "language: rust"),
		"note misses its page": strings.ReplaceAll(goodRule, "go-qa rule-doc gqaci.ast.example", "see elsewhere"),
		"no id":                strings.ReplaceAll(goodRule, "id: gqaci.ast.example\n", ""),
		"two documents":        goodRule + "---\n" + goodRule,
		"not yaml":             "id: [oops\n",
	} {
		_, err := astgrep.LoadPack(fstest.MapFS{"r.yml": {Data: []byte(text)}})
		if err == nil {
			t.Errorf("%s accepted", name)
		}
	}
}

func TestLoadPackRejectsAFileNotNamedAfterItsRule(t *testing.T) {
	t.Parallel()
	if _, err := astgrep.LoadPack(fstest.MapFS{"other.yml": {Data: []byte(goodRule)}}); err == nil {
		t.Error("misnamed file accepted")
	}
}

func TestLoadPackAcceptsTheNamedFile(t *testing.T) {
	t.Parallel()
	if _, err := astgrep.LoadPack(fstest.MapFS{"gqaci.ast.example.yml": {Data: []byte(goodRule)}}); err != nil {
		t.Error(err)
	}
}

func TestWritePackWritesOneMultiDocumentFile(t *testing.T) {
	t.Parallel()
	pack, err := astgrep.LoadPack(fstest.MapFS{
		"gqaci.ast.a.yml": {Data: []byte(strings.ReplaceAll(goodRule, "example", "a"))},
		"gqaci.ast.b.yml": {Data: []byte(strings.ReplaceAll(goodRule, "example", "b"))},
	})
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path, err := pack.Write(dir)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(path) != dir {
		t.Errorf("written outside %s: %s", dir, path)
	}
	handle, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	text, err := handle.ReadFile(filepath.Base(path))
	if err != nil {
		t.Fatal(err)
	}
	if err := handle.Close(); err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(text), "\n---\n") != 1 || !strings.Contains(string(text), "gqaci.ast.a") || !strings.Contains(string(text), "gqaci.ast.b") {
		t.Errorf("pack text:\n%s", text)
	}
}

func TestTheShippedPackIsValid(t *testing.T) {
	t.Parallel()
	if _, err := astgrep.Bundled(); err != nil {
		t.Fatal(err)
	}
}

// The documentation says the pack is empty exactly when it is: a claim about the shipped pack is
// read from the code, so filling the pack without editing the page (or the reverse) fails here.
func TestTheDocumentationSaysThePackIsEmptyExactlyWhenItIs(t *testing.T) {
	t.Parallel()
	pack, err := astgrep.Bundled()
	if err != nil {
		t.Fatal(err)
	}
	page, err := os.ReadFile(filepath.Join("..", "..", "docs", "astgrep.md"))
	if err != nil {
		t.Fatal(err)
	}
	says := strings.Contains(string(page), "**The pack is empty.**")
	if says != pack.Empty() {
		t.Errorf("docs/astgrep.md says the pack is empty: %v; the embedded pack is empty: %v", says, pack.Empty())
	}
}

func TestAnEmptyPackWritesNothing(t *testing.T) {
	t.Parallel()
	pack, err := astgrep.LoadPack(fstest.MapFS{})
	if err != nil {
		t.Fatal(err)
	}
	if !pack.Empty() {
		t.Error("not empty")
	}
	if _, err := pack.Write(t.TempDir()); err == nil {
		t.Error("an empty pack was written")
	}
}

func TestPackRulesCarryTheirMessage(t *testing.T) {
	t.Parallel()
	pack, err := astgrep.LoadPack(fstest.MapFS{"gqaci.ast.example.yml": {Data: []byte(goodRule)}})
	if err != nil {
		t.Fatal(err)
	}
	rules := pack.Rules()
	if len(rules) != 1 || rules[0].ID != "gqaci.ast.example" || rules[0].Severity != "error" || rules[0].Message != "m" {
		t.Errorf("rules = %+v", rules)
	}
}
