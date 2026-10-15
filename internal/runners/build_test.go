package runners_test

import (
	"path/filepath"
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

func TestBuildIgnoresTheCallersWorkspace(t *testing.T) {
	dir := module(t, map[string]string{"lib/lib.go": libStandalone, "go.work": "go 1.26\n\nuse ./nowhere\n"})
	if output, ok, err := runners.Build(ctx(), dir); err != nil || !ok {
		t.Errorf("a go.work that does not resolve must not matter: %q, %v, %v", output, ok, err)
	}
}

func TestBuildWithoutTheGoToolIsAnError(t *testing.T) {
	t.Setenv("PATH", filepath.Join(t.TempDir(), "empty"))
	_, _, err := runners.Build(ctx(), module(t, map[string]string{"lib/lib.go": libStandalone}))
	if err == nil || !strings.Contains(err.Error(), "go not found on PATH") {
		t.Errorf("got %v", err)
	}
}
