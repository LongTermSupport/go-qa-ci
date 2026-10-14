package astgrep_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/astgrep"
)

func write(t *testing.T, root, rel, text string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadProjectAbsentWithoutSgconfig(t *testing.T) {
	t.Parallel()
	_, found, err := astgrep.LoadProject(t.TempDir())
	if err != nil || found {
		t.Errorf("found=%v err=%v", found, err)
	}
}

func TestLoadProjectReadsRuleAndTestDirs(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write(t, root, "sgconfig.yml", "ruleDirs:\n  - rules\n  - more/rules\ntestConfigs:\n  - testDir: tests-a\n  - testDir: tests-b\nutilDirs: [utils]\n")
	project, found, err := astgrep.LoadProject(root)
	if err != nil || !found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	if want := []string{"rules", "more/rules"}; !reflect.DeepEqual(project.RuleDirs, want) {
		t.Errorf("RuleDirs = %v", project.RuleDirs)
	}
	if want := []string{"tests-a", "tests-b"}; !reflect.DeepEqual(project.TestDirs, want) {
		t.Errorf("TestDirs = %v", project.TestDirs)
	}
}

func TestLoadProjectRejectsMalformedAndEscapingConfig(t *testing.T) {
	t.Parallel()
	for name, text := range map[string]string{
		"not yaml":          "ruleDirs: [unterminated\n",
		"ruleDirs a string": "ruleDirs: rules\n",
		"dir escapes root":  "ruleDirs: [../elsewhere]\n",
		"absolute dir":      "ruleDirs: [/etc]\n",
		"test dir escapes":  "ruleDirs: [r]\ntestConfigs:\n  - testDir: ../t\n",
	} {
		root := t.TempDir()
		write(t, root, "sgconfig.yml", text)
		if _, _, err := astgrep.LoadProject(root); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

func TestRulesReadsIDsSeveritiesAndNamesUnnamedFiles(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write(t, root, "sgconfig.yml", "ruleDirs: [rules]\n")
	write(t, root, "rules/ok.yml", "id: ok\nlanguage: go\nseverity: error\nmessage: m\nrule:\n  pattern: x\n")
	write(t, root, "rules/noid.yml", "language: go\nseverity: error\nmessage: m\nrule:\n  pattern: x\n")
	write(t, root, "rules/multi.yaml", "id: a\nseverity: warning\n---\nid: b\n---\nlanguage: go\n")
	write(t, root, "rules/notes.txt", "ignored")
	project, _, err := astgrep.LoadProject(root)
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := project.Inventory(root)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, rule := range inventory.Rules {
		got[rule.ID] = rule.Severity + "@" + rule.File
	}
	want := map[string]string{"ok": "error@rules/ok.yml", "a": "warning@rules/multi.yaml", "b": "hint@rules/multi.yaml"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("rules = %v", got)
	}
	if wantUnnamed := []string{"rules/multi.yaml", "rules/noid.yml"}; !reflect.DeepEqual(inventory.Unnamed, wantUnnamed) {
		t.Errorf("unnamed = %v", inventory.Unnamed)
	}
}

func TestInventoryFailsOnAMissingRuleDirAndAnUnparseableRuleFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write(t, root, "sgconfig.yml", "ruleDirs: [rules]\n")
	project, _, err := astgrep.LoadProject(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := project.Inventory(root); err == nil {
		t.Error("missing rule dir accepted")
	}
	write(t, root, "rules/bad.yml", "id: [unterminated\n")
	if _, err := project.Inventory(root); err == nil {
		t.Error("unparseable rule file accepted")
	}
}

func TestTestedRulesNeedBothKindsOfCase(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write(t, root, "sgconfig.yml", "ruleDirs: [rules]\ntestConfigs:\n  - testDir: tests-a\n  - testDir: tests-b\n")
	write(t, root, "tests-a/one.yml", "id: both\nvalid: ['a']\ninvalid: ['b']\n")
	write(t, root, "tests-a/two.yml", "id: only-valid\nvalid: ['a']\n")
	write(t, root, "tests-b/three.yml", "id: empty-invalid\nvalid: ['a']\ninvalid: []\n")
	write(t, root, "tests-b/four.yml", "id: both\nvalid: ['c']\ninvalid: ['d']\n")
	project, _, err := astgrep.LoadProject(root)
	if err != nil {
		t.Fatal(err)
	}
	tested, err := project.Tested(root)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"both"}; !reflect.DeepEqual(tested, want) {
		t.Errorf("tested = %v", tested)
	}
}

func TestTestedRulesIsEmptyWithoutTestConfigs(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write(t, root, "sgconfig.yml", "ruleDirs: [rules]\n")
	project, _, err := astgrep.LoadProject(root)
	if err != nil {
		t.Fatal(err)
	}
	tested, err := project.Tested(root)
	if err != nil || len(tested) != 0 || project.HasTests() {
		t.Errorf("tested=%v err=%v hasTests=%v", tested, err, project.HasTests())
	}
}
