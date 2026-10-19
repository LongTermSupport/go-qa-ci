package changed_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/changed"
)

const module = "example.com/m"

// write creates a file under dir, with parents.
func write(t *testing.T, dir, rel, content string) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	full := append([]string{"-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false", "-c", "init.defaultBranch=main"}, args...)
	cmd := exec.CommandContext(t.Context(), "git", full...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// project makes a committed module with a leaf package, its importer, a package importing the
// importer, and an unrelated package.
func project(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	write(t, dir, "go.mod", "module "+module+"\n\ngo 1.26\n")
	write(t, dir, "leaf/leaf.go", "package leaf\n\nfunc Leaf() int { return 1 }\n")
	write(t, dir, "mid/mid.go", "package mid\n\nimport \""+module+"/leaf\"\n\nfunc Mid() int { return leaf.Leaf() }\n")
	write(t, dir, "top/top.go", "package top\n\nimport \""+module+"/mid\"\n\nfunc Top() int { return mid.Mid() }\n")
	write(t, dir, "other/other.go", "package other\n\nfunc Other() int { return 2 }\n")
	write(t, dir, "qa/go-qa.toml", "\n")
	git(t, dir, "init")
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-m", "initial")
	return dir
}

func compute(t *testing.T, dir, base string) *changed.Plan {
	t.Helper()
	plan, err := changed.Compute(t.Context(), changed.Options{Root: dir, Base: base, Modules: []string{""}, Widen: []string{"qa/record.toml"}})
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func TestLeafChangeRunsItsImportersButNotAnUnrelatedPackage(t *testing.T) {
	dir := project(t)
	write(t, dir, "leaf/leaf.go", "package leaf\n\nfunc Leaf() int { return 3 }\n")
	plan := compute(t, dir, "")
	if plan.Widened != "" {
		t.Fatalf("widened: %s", plan.Widened)
	}
	want := []string{"./leaf", "./mid", "./top"}
	if got := plan.Packages[""]; !slices.Equal(got, want) {
		t.Fatalf("packages %v, want %v", got, want)
	}
	if !slices.Equal(plan.Scope, []string{"leaf/leaf.go"}) {
		t.Fatalf("scope %v", plan.Scope)
	}
	if got := plan.Changed[""]; !slices.Equal(got, []string{"./leaf"}) {
		t.Fatalf("changed packages %v, want only ./leaf", got)
	}
}

func TestChangeInTheTopPackageRunsOnlyThatPackage(t *testing.T) {
	dir := project(t)
	write(t, dir, "top/top.go", "package top\n\nfunc Top() int { return 9 }\n")
	if got := compute(t, dir, "").Packages[""]; !slices.Equal(got, []string{"./top"}) {
		t.Fatalf("packages %v", got)
	}
}

func TestGoModChangeWidensAndSaysWhy(t *testing.T) {
	dir := project(t)
	write(t, dir, "go.mod", "module "+module+"\n\ngo 1.26\n\n// edited\n")
	plan := compute(t, dir, "")
	if !strings.Contains(plan.Widened, "go.mod") {
		t.Fatalf("widened reason %q does not name go.mod", plan.Widened)
	}
	if plan.Narrow() {
		t.Fatal("a widened plan must not be narrow")
	}
}

func TestEveryWideningFile(t *testing.T) {
	for _, rel := range []string{"go.sum", "sub/go.mod", ".golangci.yml", "qa/record.toml", "tools.lock", "go.work"} {
		t.Run(rel, func(t *testing.T) {
			dir := project(t)
			write(t, dir, rel, "changed\n")
			plan := compute(t, dir, "")
			if !strings.Contains(plan.Widened, rel) {
				t.Fatalf("widened reason %q does not name %s", plan.Widened, rel)
			}
		})
	}
}

func computeWith(t *testing.T, dir string, opts changed.Options) *changed.Plan {
	t.Helper()
	opts.Root, opts.Modules = dir, []string{""}
	plan, err := changed.Compute(t.Context(), opts)
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func TestADirectoryWidensOnlyWhenTheConfigurationNamesIt(t *testing.T) {
	dir := project(t)
	write(t, dir, "qa/rules/r.yml", "changed\n")
	if plan := computeWith(t, dir, changed.Options{}); plan.Widened != "" {
		t.Fatalf("qa/ widened with no configuration naming it: %s", plan.Widened)
	}
	plan := computeWith(t, dir, changed.Options{WidenDirs: []string{"qa/rules"}})
	if !strings.Contains(plan.Widened, "qa/rules/r.yml") {
		t.Fatalf("widened reason %q does not name the file under the configured directory", plan.Widened)
	}
}

func TestAWideningDirectoryDoesNotMatchASiblingWithTheSamePrefix(t *testing.T) {
	dir := project(t)
	write(t, dir, "qa2/r.yml", "changed\n")
	if plan := computeWith(t, dir, changed.Options{WidenDirs: []string{"qa"}}); plan.Widened != "" {
		t.Fatalf("qa2/ widened a run configured for qa/: %s", plan.Widened)
	}
}

func TestABaseThatLooksLikeAnOptionIsRefusedBeforeGitSeesIt(t *testing.T) {
	dir := project(t)
	_, err := changed.Compute(t.Context(), changed.Options{Root: dir, Base: "--all", Modules: []string{""}})
	if err == nil || !strings.Contains(err.Error(), "not an option") {
		t.Fatalf("a base starting with a dash was not refused as an option: %v", err)
	}
}

func TestBaseCoversACommittedChangeOnABranch(t *testing.T) {
	dir := project(t)
	git(t, dir, "checkout", "-b", "feature")
	write(t, dir, "mid/mid.go", "package mid\n\nimport \""+module+"/leaf\"\n\nfunc Mid() int { return leaf.Leaf() + 1 }\n")
	git(t, dir, "commit", "-am", "change mid")
	if got := compute(t, dir, "").Packages; len(got) != 0 {
		t.Fatalf("against HEAD nothing is changed, got %v", got)
	}
	plan := compute(t, dir, "main")
	if got := plan.Packages[""]; !slices.Equal(got, []string{"./mid", "./top"}) {
		t.Fatalf("packages %v", got)
	}
}

func TestBaseAlsoCoversTheWorkingTree(t *testing.T) {
	dir := project(t)
	git(t, dir, "checkout", "-b", "feature")
	write(t, dir, "other/other.go", "package other\n\nfunc Other() int { return 5 }\n")
	if got := compute(t, dir, "main").Packages[""]; !slices.Equal(got, []string{"./other"}) {
		t.Fatalf("packages %v", got)
	}
}

func TestBaseUsesTheMergeBaseNotTheTipOfTheBase(t *testing.T) {
	dir := project(t)
	git(t, dir, "checkout", "-b", "feature")
	write(t, dir, "other/other.go", "package other\n\nfunc Other() int { return 5 }\n")
	git(t, dir, "commit", "-am", "feature change")
	git(t, dir, "checkout", "main")
	write(t, dir, "leaf/leaf.go", "package leaf\n\nfunc Leaf() int { return 7 }\n")
	git(t, dir, "commit", "-am", "main moved on")
	git(t, dir, "checkout", "feature")
	if got := compute(t, dir, "main").Packages[""]; !slices.Equal(got, []string{"./other"}) {
		t.Fatalf("packages %v, want only the feature branch's change", got)
	}
}

func TestUntrackedFileIsIncludedButIgnoredIsNot(t *testing.T) {
	dir := project(t)
	write(t, dir, ".gitignore", "ignored/\n")
	git(t, dir, "add", ".gitignore")
	git(t, dir, "commit", "-m", "ignore")
	write(t, dir, "fresh/fresh.go", "package fresh\n\nfunc Fresh() int { return 1 }\n")
	write(t, dir, "ignored/ignored.go", "package ignored\n")
	plan := compute(t, dir, "")
	if !slices.Equal(plan.Files, []string{"fresh/fresh.go"}) {
		t.Fatalf("files %v", plan.Files)
	}
	if got := plan.Packages[""]; !slices.Equal(got, []string{"./fresh"}) {
		t.Fatalf("packages %v", got)
	}
}

func TestNothingChangedIsEmptyAndNarrow(t *testing.T) {
	plan := compute(t, project(t), "")
	if !plan.Narrow() || len(plan.Files) != 0 || len(plan.Packages) != 0 || len(plan.Scope) != 0 {
		t.Fatalf("plan %+v", plan)
	}
}

func TestDeletedFileStillAffectsItsImporters(t *testing.T) {
	dir := project(t)
	write(t, dir, "leaf/extra.go", "package leaf\n")
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-m", "extra")
	if err := os.Remove(filepath.Join(dir, "leaf", "extra.go")); err != nil {
		t.Fatal(err)
	}
	plan := compute(t, dir, "")
	if len(plan.Scope) != 0 {
		t.Fatalf("a deleted file cannot be scanned, scope %v", plan.Scope)
	}
	if got := plan.Packages[""]; !slices.Equal(got, []string{"./leaf", "./mid", "./top"}) {
		t.Fatalf("packages %v", got)
	}
}

func TestRemovedPackageWidens(t *testing.T) {
	dir := project(t)
	if err := os.RemoveAll(filepath.Join(dir, "other")); err != nil {
		t.Fatal(err)
	}
	if plan := compute(t, dir, ""); !strings.Contains(plan.Widened, "other/other.go") {
		t.Fatalf("widened reason %q", plan.Widened)
	}
}

func TestNonGoChangeNeedsNoPackages(t *testing.T) {
	dir := project(t)
	write(t, dir, "notes.md", "hello\n")
	plan := compute(t, dir, "")
	if !plan.Narrow() || len(plan.Packages) != 0 || !slices.Equal(plan.Scope, []string{"notes.md"}) {
		t.Fatalf("plan %+v", plan)
	}
}

func TestTestdataFileRunsOnlyThePackageThatOwnsIt(t *testing.T) {
	dir := project(t)
	write(t, dir, "leaf/testdata/x.go", "package x\n")
	write(t, dir, "leaf/testdata/golden/out.txt", "golden\n")
	plan := compute(t, dir, "")
	if !plan.Narrow() || !slices.Equal(plan.Packages[""], []string{"./leaf"}) || !slices.Equal(plan.Changed[""], []string{"./leaf"}) {
		t.Fatalf("plan %+v", plan)
	}
}

func TestChangedTestOnlyPackageIsRunButHasNoCodeToMeasure(t *testing.T) {
	dir := project(t)
	write(t, dir, "golden/golden_test.go", "package golden\n\nimport \"testing\"\n\nfunc TestG(t *testing.T) {}\n")
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-m", "golden")
	write(t, dir, "golden/golden_test.go", "package golden\n\nimport \"testing\"\n\nfunc TestG(t *testing.T) { t.Log(1) }\n")
	write(t, dir, "other/other.go", "package other\n\nfunc Other() int { return 5 }\n")
	plan := compute(t, dir, "")
	if !slices.Equal(plan.Packages[""], []string{"./golden", "./other"}) || !slices.Equal(plan.Changed[""], []string{"./other"}) {
		t.Fatalf("packages %v, changed %v", plan.Packages, plan.Changed)
	}
}

func TestEmbeddedFileRunsItsPackageAndItsImporters(t *testing.T) {
	dir := project(t)
	write(t, dir, "leaf/text.txt", "one\n")
	write(t, dir, "leaf/leaf.go", "package leaf\n\nimport _ \"embed\"\n\n//go:embed text.txt\nvar Text string\n\nfunc Leaf() int { return 1 }\n")
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-m", "embed")
	write(t, dir, "leaf/text.txt", "two\n")
	write(t, dir, "other/readme.md", "not embedded\n")
	plan := compute(t, dir, "")
	if !plan.Narrow() || !slices.Equal(plan.Packages[""], []string{"./leaf", "./mid", "./top"}) {
		t.Fatalf("plan %+v", plan)
	}
}

func TestSecondModuleReportsItsOwnPackages(t *testing.T) {
	dir := project(t)
	write(t, dir, "nested/go.mod", "module example.com/nested\n\ngo 1.26\n")
	write(t, dir, "nested/n/n.go", "package n\n")
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-m", "nested")
	write(t, dir, "nested/n/n.go", "package n\n\nvar X = 1\n")
	plan, err := changed.Compute(t.Context(), changed.Options{Root: dir, Modules: []string{"", "nested"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := plan.Packages["nested"]; !slices.Equal(got, []string{"./n"}) {
		t.Fatalf("packages %v", plan.Packages)
	}
}

func TestOutsideGitIsAnError(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "go.mod", "module "+module+"\n\ngo 1.26\n")
	if _, err := changed.Compute(t.Context(), changed.Options{Root: dir, Modules: []string{""}}); err == nil {
		t.Fatal("want an error outside a git work tree")
	}
}

func TestUnknownBaseIsAnError(t *testing.T) {
	if _, err := changed.Compute(t.Context(), changed.Options{Root: project(t), Base: "no-such-ref", Modules: []string{""}}); err == nil {
		t.Fatal("want an error for an unknown base")
	}
}

func TestDescribeNamesModeAndScope(t *testing.T) {
	dir := project(t)
	write(t, dir, "leaf/leaf.go", "package leaf\n\nfunc Leaf() int { return 3 }\n")
	got := compute(t, dir, "").Describe()
	for _, want := range []string{"changed", "HEAD", "1 changed file", "./leaf", "./mid"} {
		if !strings.Contains(got, want) {
			t.Errorf("description lacks %q:\n%s", want, got)
		}
	}
	main := compute(t, dir, "main").Describe()
	if !strings.Contains(main, "merge base of main and HEAD") {
		t.Errorf("description %q", main)
	}
	write(t, dir, "go.mod", "module "+module+"\n\ngo 1.26\n\n// e\n")
	if wide := compute(t, dir, "").Describe(); !strings.Contains(wide, "whole project") || !strings.Contains(wide, "go.mod") {
		t.Errorf("description %q", wide)
	}
	clean := compute(t, project(t), "").Describe()
	if !strings.Contains(clean, "no changed files") {
		t.Errorf("description %q", clean)
	}
}
