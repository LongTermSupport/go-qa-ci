package runners_test

import (
	"strings"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/runners"
)

const passingSource = "package fixture\n\nfunc Double(n int) int { return n * 2 }\n"

func TestTestPassesAGreenModule(t *testing.T) {
	dir := module(t, map[string]string{
		"f.go":      passingSource,
		"f_test.go": "package fixture\n\nimport \"testing\"\n\nfunc TestDouble(t *testing.T) {\n\tif Double(2) != 4 {\n\t\tt.Fatal(\"bad\")\n\t}\n}\n",
	})
	output, passed, err := runners.Test(ctx(), dir)
	if err != nil || !passed {
		t.Errorf("got %q, %v, %v", output, passed, err)
	}
}

func TestTestFailsARedModuleAndKeepsTheOutput(t *testing.T) {
	dir := module(t, map[string]string{
		"f.go":      passingSource,
		"f_test.go": "package fixture\n\nimport \"testing\"\n\nfunc TestDouble(t *testing.T) {\n\tt.Fatal(\"deliberate failure\")\n}\n",
	})
	output, passed, err := runners.Test(ctx(), dir)
	if err != nil || passed || !strings.Contains(output, "deliberate failure") {
		t.Errorf("got %q, %v, %v", output, passed, err)
	}
}
