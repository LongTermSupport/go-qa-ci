package driver_test

import (
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis"

	"github.com/LongTermSupport/go-qa-ci/driver"
)

const serveEnv = "GQACI_DRIVER_TEST_SERVE"

var example = &analysis.Analyzer{
	Name: "example_rule",
	Doc:  "reports nothing\n\nAn example analyzer for the driver test.",
	Run:  func(*analysis.Pass) (any, error) { return struct{}{}, nil },

	ResultType: reflect.TypeFor[struct{}](),
}

// TestMain runs the test binary as a vet tool built with driver.Main when asked to.
func TestMain(m *testing.M) {
	if os.Getenv(serveEnv) == "1" {
		driver.Main(example)
	}
	os.Exit(m.Run())
}

func serve(t *testing.T, args ...string) string {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(t.Context(), exe, args...)
	cmd.Env = append(os.Environ(), serveEnv+"=1")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("%v: %v", args, err)
	}
	return string(out)
}

func TestMainRegistersTheAnalyzersAsAVetTool(t *testing.T) {
	if flags := serve(t, "-flags"); !strings.Contains(flags, `"Name": "example_rule"`) {
		t.Errorf("-flags does not list the analyzer:\n%s", flags)
	}
	if help := serve(t, "help", "example_rule"); !strings.Contains(help, "An example analyzer for the driver test.") {
		t.Errorf("help does not print the documentation:\n%s", help)
	}
}
