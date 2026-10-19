package runners_test

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/runners"
)

const (
	libUsingHelper = "package lib\n\nfunc Value() int { return helper() }\n"
	libTestHelper  = "package lib\n\nimport \"testing\"\n\nfunc helper() int { return 1 }\n\nfunc TestValue(t *testing.T) {\n\tif Value() != 1 {\n\t\tt.Fatal(\"bad\")\n\t}\n}\n"
	libStandalone  = "package lib\n\nfunc Value() int { return 1 }\n"
)

func TestBuildPassesAModuleWhoseProductionFilesStandAlone(t *testing.T) {
	dir := module(t, map[string]string{"lib/lib.go": libStandalone, "lib/lib_test.go": "package lib\n\nimport \"testing\"\n\nfunc TestValue(t *testing.T) {}\n"})
	output, ok, err := runners.Build(ctx(), dir)
	if err != nil || !ok {
		t.Errorf("got %q, %v, %v", output, ok, err)
	}
}

func TestBuildFailsAProductionFileThatCallsATestOnlyHelperAndNamesThePackage(t *testing.T) {
	dir := module(t, map[string]string{"lib/lib.go": libUsingHelper, "lib/lib_test.go": libTestHelper})
	// The tests of this package compile and pass, which is why the other gates are blind to it.
	if testOutput, passed, err := runners.Test(ctx(), dir); err != nil || !passed {
		t.Fatalf("the fixture must pass its own tests: %q, %v, %v", testOutput, passed, err)
	}
	output, ok, err := runners.Build(ctx(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if ok || !strings.Contains(output, "example.com/fixture/lib") || !strings.Contains(output, "undefined: helper") {
		t.Errorf("got %q, %v", output, ok)
	}
}

func TestBuildLimitedToPackagesLeavesTheOthersUnbuilt(t *testing.T) {
	dir := module(t, map[string]string{"lib/lib.go": libUsingHelper, "lib/lib_test.go": libTestHelper, "good/good.go": "package good\n"})
	output, ok, err := runners.Build(ctx(), dir, "./good")
	if err != nil || !ok {
		t.Errorf("got %q, %v, %v", output, ok, err)
	}
}

// goldenTest is a test-only package: it has no production file, so go build refuses it when it is
// named although ./... skips it. Its test files are judged by the test stage, not by the build.
const goldenTest = "package golden\n\nimport (\n\t\"testing\"\n\n\t\"example.com/fixture/lib\"\n)\n\nfunc TestValue(t *testing.T) {\n\tif lib.Value() != 1 {\n\t\tt.Fatal(\"bad\")\n\t}\n}\n"

func TestBuildNamingATestOnlyPackageBuildsTheOthersAndPasses(t *testing.T) {
	dir := module(t, map[string]string{"lib/lib.go": libStandalone, "golden/golden_test.go": goldenTest})
	for _, patterns := range [][]string{{"./golden", "./lib"}, {"./golden"}} {
		if output, ok, err := runners.Build(ctx(), dir, patterns...); err != nil || !ok {
			t.Errorf("%v: got %q, %v, %v", patterns, output, ok, err)
		}
	}
}

func TestBuildNamingATestOnlyPackageStillFailsABrokenProductionPackage(t *testing.T) {
	dir := module(t, map[string]string{"lib/lib.go": libUsingHelper, "lib/lib_test.go": libTestHelper, "golden/golden_test.go": goldenTest})
	output, ok, err := runners.Build(ctx(), dir, "./golden", "./lib")
	if err != nil {
		t.Fatal(err)
	}
	if ok || !strings.Contains(output, "undefined: helper") {
		t.Errorf("got %q, %v", output, ok)
	}
}

func TestBuildNamingAPackageThatDoesNotLoadFails(t *testing.T) {
	cases := map[string]map[string]string{
		"missing directory":       {"lib/lib.go": libStandalone},
		"no Go files":             {"lib/lib.go": libStandalone, "missing/README": "not Go\n"},
		"constraints exclude all": {"lib/lib.go": libStandalone, "missing/x.go": "//go:build never\n\npackage missing\n"},
		"syntax error":            {"lib/lib.go": libStandalone, "missing/x.go": "package missing\n\nfunc {\n"},
	}
	for name, files := range cases {
		dir := module(t, files)
		output, ok, err := runners.Build(ctx(), dir, "./lib", "./missing")
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if ok || !strings.Contains(output, "missing") {
			t.Errorf("%s: a package that does not load must fail the build: %q, %v", name, output, ok)
		}
	}
}

func TestBuildBuildsAWorkspaceModuleUnderTheWorkspace(t *testing.T) {
	dir := module(t, map[string]string{
		"go.work":   "go 1.26\n\nuse (\n\t./ma\n\t./mb\n)\n",
		"go.mod":    "module example.com/unused\n\ngo 1.26\n",
		"ma/go.mod": "module example.com/ma\n\ngo 1.26\n",
		"ma/ma.go":  "package ma\n\nimport \"example.com/mb\"\n\nfunc A() int { return mb.B() }\n",
		"mb/go.mod": "module example.com/mb\n\ngo 1.26\n",
		"mb/mb.go":  "package mb\n\nfunc B() int { return 1 }\n",
	})
	if output, ok, err := runners.Build(ctx(), filepath.Join(dir, "ma")); err != nil || !ok {
		t.Errorf("a module that relies on the workspace must build under it: %q, %v, %v", output, ok, err)
	}
}

func snapshot(t *testing.T, dir string) map[string]string {
	t.Helper()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := root.Close(); closeErr != nil {
			t.Error(closeErr)
		}
	}()
	got := map[string]string{}
	err = fs.WalkDir(root.FS(), ".", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			return walkErr
		}
		data, readErr := fs.ReadFile(root.FS(), path)
		got[path] = string(data)
		if readErr != nil {
			return fmt.Errorf("reading %s: %w", path, readErr)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestBuildLeavesNoBinaryBehindForASingleMainPackage(t *testing.T) {
	dir := module(t, map[string]string{"main.go": "package main\n\nfunc main() {}\n", "cmd/x/main.go": "package main\n\nfunc main() {}\n"})
	before := snapshot(t, dir)
	for _, patterns := range [][]string{nil, {"./cmd/x"}} {
		if output, ok, err := runners.Build(ctx(), dir, patterns...); err != nil || !ok {
			t.Fatalf("%v: %q, %v, %v", patterns, output, ok, err)
		}
	}
	if after := snapshot(t, dir); !reflect.DeepEqual(before, after) {
		t.Errorf("the build changed the module directory: before %d files, after %d", len(before), len(after))
	}
}

func TestBuildNeverRewritesGoModOrGoSumAndDoesNotFetch(t *testing.T) {
	dir := module(t, map[string]string{"lib/lib.go": "package lib\n\nimport _ \"golang.org/x/text/language\"\n"})
	t.Setenv("GOFLAGS", "-mod=mod")
	t.Setenv("GOPROXY", "off")
	before := snapshot(t, dir)
	output, ok, err := runners.Build(ctx(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if ok || !strings.Contains(output, "no required module provides") {
		t.Errorf("a missing requirement must fail the build, not be fetched: %q, %v", output, ok)
	}
	if after := snapshot(t, dir); !reflect.DeepEqual(before, after) {
		t.Error("go.mod or go.sum or another file was changed by the build")
	}
}

func TestBuildFailsAVendoredProjectThatImportsWhatIsNotVendored(t *testing.T) {
	dir := module(t, map[string]string{
		"go.mod":                        "module example.com/fixture\n\ngo 1.26\n\nrequire example.com/dep v0.0.0\n\nreplace example.com/dep => ./dep\n",
		"dep/go.mod":                    "module example.com/dep\n\ngo 1.26\n",
		"dep/a/a.go":                    "package a\n",
		"dep/b/b.go":                    "package b\n",
		"vendor/modules.txt":            "# example.com/dep v0.0.0 => ./dep\n## explicit; go 1.26\nexample.com/dep/a\n# example.com/dep => ./dep\n",
		"vendor/example.com/dep/a/a.go": "package a\n",
		"lib/lib.go":                    "package lib\n\nimport _ \"example.com/dep/b\"\n",
	})
	t.Setenv("GOFLAGS", "-mod=mod")
	output, ok, err := runners.Build(ctx(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if ok || !strings.Contains(output, "example.com/dep/b") {
		t.Errorf("a package missing from vendor must fail the build: %q, %v", output, ok)
	}
}

func TestBuildWithoutTheGoToolIsAnError(t *testing.T) {
	t.Setenv("PATH", filepath.Join(t.TempDir(), "empty"))
	_, _, err := runners.Build(ctx(), module(t, map[string]string{"lib/lib.go": libStandalone}))
	if err == nil || !strings.Contains(err.Error(), "go not found on PATH") {
		t.Errorf("got %v", err)
	}
}

func TestBuildKeepsTheFlagsSetWithGoEnvW(t *testing.T) {
	goenv := filepath.Join(t.TempDir(), "env")
	if err := os.WriteFile(goenv, []byte("GOFLAGS=-tags=special -mod=mod\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOENV", goenv)
	t.Setenv("GOFLAGS", "")
	if err := os.Unsetenv("GOFLAGS"); err != nil {
		t.Fatal(err)
	}
	dir := module(t, map[string]string{"lib/lib.go": "package lib\n", "lib/special.go": "//go:build special\n\npackage lib\n\nvar _ = missing\n"})
	output, ok, err := runners.Build(ctx(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if ok || !strings.Contains(output, "undefined: missing") {
		t.Errorf("the tags of go env -w must reach the build: %q, %v", output, ok)
	}
}
