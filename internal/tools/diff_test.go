package tools_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/changed"
	"github.com/LongTermSupport/go-qa-ci/internal/config"
	"github.com/LongTermSupport/go-qa-ci/internal/pipeline"
	"github.com/LongTermSupport/go-qa-ci/internal/tools"
)

const (
	diffMod        = "example.com/diff"
	leafSource     = "// Package leaf is a fixture.\npackage leaf\n\n// Leaf is a fixture.\nfunc Leaf() int { return 1 }\n"
	leafTest       = "package leaf\n\nimport \"testing\"\n\nfunc TestLeaf(t *testing.T) {\n\tif Leaf() != 1 {\n\t\tt.Fatal(\"bad\")\n\t}\n}\n"
	midSource      = "// Package mid is a fixture.\npackage mid\n\nimport \"" + diffMod + "/leaf\"\n\n// Mid is a fixture.\nfunc Mid() int { return leaf.Leaf() }\n"
	midTest        = "package mid\n\nimport \"testing\"\n\nfunc TestMid(t *testing.T) {\n\tif Mid() != 1 {\n\t\tt.Fatal(\"bad\")\n\t}\n}\n"
	otherSource    = "// Package other is a fixture.\npackage other\n\n// Other is a fixture.\nfunc Other() int { return 2 }\n\n// Untested is never called by a test.\nfunc Untested() int { return 3 }\n"
	otherTest      = "package other\n\nimport \"testing\"\n\nfunc TestOther(t *testing.T) {\n\tt.Fatal(\"the unrelated package must not run\")\n}\n"
	otherUnsafe    = "// Package other is a fixture.\npackage other\n\nimport \"os\"\n\n// Clean removes a file.\nfunc Clean() {\n\tos.Remove(\"x\")\n}\n"
	midUnsafe      = "// Package mid is a fixture.\npackage mid\n\nimport (\n\t\"os\"\n\n\t\"" + diffMod + "/leaf\"\n)\n\n// Mid is a fixture.\nfunc Mid() int { return leaf.Leaf() }\n\n// Clean removes a file.\nfunc Clean() {\n\tos.Remove(\"x\")\n}\n"
	leafChanged    = "// Package leaf is a fixture.\npackage leaf\n\n// Leaf is a fixture.\nfunc Leaf() int { return 1 + 0 }\n"
	nolintSource   = "// Package other is a fixture.\npackage other\n\nimport \"os\"\n\n// Clean removes a file.\nfunc Clean() {\n\tos.Remove(\"x\") //nolint:errcheck // fixture\n}\n"
	otherForbidden = "// Package other is a fixture.\npackage other\n\n// Forbidden is forbidden.\nfunc Forbidden() {}\n\n// F calls it.\nfunc F() { Forbidden() }\n"
	goModEdited    = "module " + diffMod + "\n\ngo 1.26\n\n// edited\n"
	diffThreshold  = 80
)

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	full := append([]string{"-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false", "-c", "init.defaultBranch=main"}, args...)
	cmd := exec.CommandContext(t.Context(), "git", full...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func put(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// diffProject commits a module of a leaf, an importer of it and an unrelated package, with the
// given extra files, and returns its root.
func diffProject(t *testing.T, extra map[string]string) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"go.mod":              "module " + diffMod + "\n\ngo 1.26\n",
		"leaf/leaf.go":        leafSource,
		"leaf/leaf_test.go":   leafTest,
		"mid/mid.go":          midSource,
		"mid/mid_test.go":     midTest,
		"other/other.go":      otherSource,
		"other/other_test.go": otherTest,
	}
	for name, content := range extra {
		files[name] = content
	}
	put(t, root, files)
	runGit(t, root, "init")
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-m", "initial")
	return root
}

// narrowed is an executor for root whose run is limited to what changed since the commit.
func narrowed(t *testing.T, root string) (tools.Executor, *bytes.Buffer) {
	t.Helper()
	plan, err := changed.Compute(t.Context(), changed.Options{Root: root, Modules: []string{""}})
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	cfg := config.Default()
	cfg.Coverage.FailUnderStatements = diffThreshold
	return tools.Executor{Ctx: tools.Context{Root: root, Config: cfg, Today: fixedToday(t), Diff: plan}, Stderr: &stderr}, &stderr
}

func TestDiffTestRunsTheChangedPackageAndItsImportersNotAnUnrelatedOne(t *testing.T) {
	root := diffProject(t, nil)
	put(t, root, map[string]string{"leaf/leaf.go": leafChanged})
	exec, stderr := narrowed(t, root)
	if out := exec.Run(stepNamed(t, "test")); out.Kind != pipeline.KindPassed {
		t.Fatalf("an unrelated package's failing test must not run: %+v\n%s", out, stderr)
	}
}

func TestDiffTestRunsAnImporterOfTheChangedPackage(t *testing.T) {
	root := diffProject(t, nil)
	put(t, root, map[string]string{
		"leaf/leaf.go":    leafChanged,
		"mid/mid_test.go": strings.Replace(midTest, "!= 1", "!= 99", 1),
	})
	exec, stderr := narrowed(t, root)
	out := exec.Run(stepNamed(t, "test"))
	if out.Kind != pipeline.KindFailed || !strings.Contains(stderr.String(), "TestMid") {
		t.Fatalf("an importer's test must run: %+v\n%s", out, stderr)
	}
	if strings.Contains(stderr.String(), "TestOther") {
		t.Errorf("the unrelated package ran:\n%s", stderr)
	}
}

func TestDiffGolangciLintsTheChangedPackagesAndImportersOnly(t *testing.T) {
	root := diffProject(t, map[string]string{"other/other.go": otherUnsafe})
	put(t, root, map[string]string{"leaf/leaf.go": leafChanged})
	exec, stderr := narrowed(t, root)
	if out := exec.Run(stepNamed(t, "golangci")); out.Kind != pipeline.KindPassed {
		t.Fatalf("the unrelated package's findings must not be reported: %+v\n%s", out, stderr)
	}
	put(t, root, map[string]string{"mid/mid.go": midUnsafe})
	exec, stderr = narrowed(t, root)
	if out := exec.Run(stepNamed(t, "golangci")); out.Kind != pipeline.KindFailed || !strings.Contains(stderr.String(), "mid/mid.go") {
		t.Fatalf("an importer's findings must be reported: %+v\n%s", out, stderr)
	}
}

func TestDiffAnalyzersVetsTheChangedPackagesAndImportersOnly(t *testing.T) {
	root := diffProject(t, map[string]string{"other/other.go": otherForbidden})
	put(t, root, map[string]string{"leaf/leaf.go": leafChanged})
	exec, stderr := narrowed(t, root)
	if out := exec.Run(stepNamed(t, "analyzers")); out.Kind != pipeline.KindPassed {
		t.Fatalf("the unrelated package's findings must not be reported: %+v\n%s", out, stderr)
	}
	put(t, root, map[string]string{"other/other.go": strings.Replace(otherForbidden, "func F()", "func G()", 1)})
	exec, stderr = narrowed(t, root)
	if out := exec.Run(stepNamed(t, "analyzers")); out.Kind != pipeline.KindFailed || !strings.Contains(stderr.String(), "error[gqaci_test]") {
		t.Fatalf("a changed package's findings must be reported: %+v\n%s", out, stderr)
	}
}

func TestDiffCoverageMeasuresOnlyTheAffectedPackages(t *testing.T) {
	root := diffProject(t, nil)
	put(t, root, map[string]string{"leaf/leaf.go": leafChanged})
	exec, stderr := narrowed(t, root)
	if out := exec.Run(stepNamed(t, "coverage")); out.Kind != pipeline.KindPassed {
		t.Fatalf("code outside the affected packages must not count against coverage: %+v\n%s", out, stderr)
	}
}

func TestDiffSuppressionScansOnlyTheChangedFiles(t *testing.T) {
	root := diffProject(t, map[string]string{"other/other.go": nolintSource})
	put(t, root, map[string]string{"leaf/leaf.go": leafChanged})
	exec, stderr := narrowed(t, root)
	if out := exec.Run(stepNamed(t, "suppression")); out.Kind != pipeline.KindPassed {
		t.Fatalf("an unchanged file's directive must not be scanned: %+v\n%s", out, stderr)
	}
	put(t, root, map[string]string{"leaf/leaf.go": strings.Replace(leafChanged, "return 1 + 0", "return 1 //nolint:all // fixture", 1) + "\n"})
	exec, stderr = narrowed(t, root)
	if out := exec.Run(stepNamed(t, "suppression")); out.Kind != pipeline.KindFailed {
		t.Fatalf("a changed file's directive must be reported: %+v\n%s", out, stderr)
	}
}

func TestDiffRunsVulnAndModWhenAGoPackageChangedBecauseCallsAndImportsCanChange(t *testing.T) {
	root := diffProject(t, nil)
	put(t, root, map[string]string{"leaf/leaf.go": leafChanged})
	exec, stderr := narrowed(t, root)
	for _, name := range []string{"vuln", "mod"} {
		if out := exec.Run(stepNamed(t, name)); out.Kind == pipeline.KindSkipped {
			t.Errorf("%s was skipped although a Go package changed: %+v\n%s", name, out, stderr)
		}
	}
}

func TestDiffWidenedRunCoversEveryPackage(t *testing.T) {
	root := diffProject(t, nil)
	put(t, root, map[string]string{"go.mod": goModEdited})
	exec, stderr := narrowed(t, root)
	out := exec.Run(stepNamed(t, "test"))
	if out.Kind != pipeline.KindFailed || !strings.Contains(stderr.String(), "TestOther") {
		t.Fatalf("a widened run covers the unrelated package: %+v\n%s", out, stderr)
	}
}

func TestDiffWithNothingChangedSkipsWhatIsScopedAndStillRunsTheRepoWideDefences(t *testing.T) {
	root := diffProject(t, nil)
	exec, stderr := narrowed(t, root)
	for _, name := range []string{"fmt", "suppression", "ast-grep", "golangci", "analyzers", "deadcode", "vuln", "mod", "build", "test", "coverage", "mutants"} {
		if out := exec.Run(stepNamed(t, name)); out.Kind != pipeline.KindSkipped {
			t.Errorf("%s: want skipped, got %+v", name, out)
		}
	}
	for _, name := range []string{"record", "docs", "summary"} {
		if out := exec.Run(stepNamed(t, name)); out.Kind == pipeline.KindSkipped {
			t.Errorf("%s: a repo-wide defence must run, got %+v", name, out)
		}
	}
	if !strings.Contains(stderr.String(), "summary: no [summary] file is configured") {
		t.Errorf("the summary check did not run:\n%s", stderr)
	}
}

func TestDiffRecordStillJudgesTheWholeRecord(t *testing.T) {
	root := diffProject(t, map[string]string{"qa/record.toml": "[[exception]]\nrule = \"errcheck\"\npath = \"a.go\"\njustification = \"legacy\"\n"})
	put(t, root, map[string]string{"notes.md": "x\n"})
	exec, stderr := narrowed(t, root)
	if out := exec.Run(stepNamed(t, "record")); out.Kind != pipeline.KindFailed || !strings.Contains(stderr.String(), "gqaci.record") {
		t.Fatalf("got %+v\n%s", out, stderr)
	}
}

func TestDiffBuildCoversTheChangedPackageAndItsImportersNotAnUnrelatedOne(t *testing.T) {
	root := diffProject(t, map[string]string{"other/broken.go": "package other\n\nfunc Broken() int { return testOnly() }\n"})
	put(t, root, map[string]string{"leaf/leaf.go": leafChanged})
	exec, stderr := narrowed(t, root)
	if out := exec.Run(stepNamed(t, "build")); out.Kind != pipeline.KindPassed {
		t.Fatalf("an unrelated package must not be built: %+v\n%s", out, stderr)
	}
	put(t, root, map[string]string{"leaf/leaf.go": "package leaf\n\nfunc Leaf() int { return missing() }\n"})
	exec, stderr = narrowed(t, root)
	if out := exec.Run(stepNamed(t, "build")); out.Kind != pipeline.KindFailed || !strings.Contains(stderr.String(), diffMod+"/leaf") {
		t.Fatalf("the changed package must be built: %+v\n%s", out, stderr)
	}
}

func TestDiffBuildFailsAnImporterThatNoLongerCompilesAgainstTheChangedPackage(t *testing.T) {
	root := diffProject(t, nil)
	put(t, root, map[string]string{"leaf/leaf.go": strings.Replace(leafSource, "Leaf()", "Renamed()", 1)})
	exec, stderr := narrowed(t, root)
	if out := exec.Run(stepNamed(t, "build")); out.Kind != pipeline.KindFailed || !strings.Contains(stderr.String(), diffMod+"/mid") {
		t.Fatalf("the importer must be built and named: %+v\n%s", out, stderr)
	}
}

func TestDiffBuildPassesWhenATestOnlyImporterOfTheChangedPackageIsAffected(t *testing.T) {
	golden := "package golden\n\nimport (\n\t\"testing\"\n\n\t\"" + diffMod + "/leaf\"\n)\n\nfunc TestGolden(t *testing.T) {\n\tif leaf.Leaf() != 1 {\n\t\tt.Fatal(\"bad\")\n\t}\n}\n"
	root := diffProject(t, map[string]string{"golden/golden_test.go": golden})
	put(t, root, map[string]string{"leaf/leaf.go": leafChanged})
	exec, stderr := narrowed(t, root)
	if out := exec.Run(stepNamed(t, "build")); out.Kind != pipeline.KindPassed {
		t.Fatalf("a test-only importer has nothing to build and must not fail the build: %+v\n%s", out, stderr)
	}
}
