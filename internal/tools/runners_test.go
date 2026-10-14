package tools_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/config"
	"github.com/LongTermSupport/go-qa-ci/internal/faketool"
	"github.com/LongTermSupport/go-qa-ci/internal/pipeline"
	"github.com/LongTermSupport/go-qa-ci/internal/tools"
)

const (
	fixtureMod  = "module example.com/fixture\n\ngo 1.26\n"
	mainSource  = "package main\n\nfunc main() {}\n\nfunc unused() {}\n"
	libSource   = "package fixture\n\nfunc Used() int { return 1 }\n\nfunc Never() int { return 2 }\n"
	passingTest = "package fixture\n\nimport \"testing\"\n\nfunc TestUsed(t *testing.T) {\n\tif Used() != 1 {\n\t\tt.Fatal(\"bad\")\n\t}\n}\n"
	failingTest = "package fixture\n\nimport \"testing\"\n\nfunc TestUsed(t *testing.T) {\n\tt.Fatal(\"deliberate failure\")\n}\n"
)

// project writes files into the executor's root.
func project(t *testing.T, exec tools.Executor, files map[string]string) {
	t.Helper()
	for name, content := range files {
		path := filepath.Join(exec.Ctx.Root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCoverageGapBelowTheThresholdFailsWithErrorCoverage(t *testing.T) {
	exec, stderr := executorFor(t, "")
	project(t, exec, map[string]string{"go.mod": fixtureMod, "f.go": libSource, "f_test.go": passingTest})
	out := exec.Run(stepNamed(t, "coverage"))
	got := stderr.String()
	if out.Kind != pipeline.KindFailed || !strings.Contains(got, "error[coverage]: .: statement coverage 50.0% is below fail_under_statements = 80") || !strings.Contains(got, "Never") {
		t.Errorf("got %+v, %q", out, got)
	}
}

func TestCoverageAtTheThresholdPasses(t *testing.T) {
	exec, stderr := executorFor(t, "")
	exec.Ctx.Config.Coverage.FailUnderStatements = 50
	project(t, exec, map[string]string{"go.mod": fixtureMod, "f.go": libSource, "f_test.go": passingTest})
	if out := exec.Run(stepNamed(t, "coverage")); out.Kind != pipeline.KindPassed || stderr.Len() != 0 {
		t.Errorf("got %+v, %q", out, stderr)
	}
}

func TestCoverageNamesFailingTests(t *testing.T) {
	exec, stderr := executorFor(t, "")
	project(t, exec, map[string]string{"go.mod": fixtureMod, "f.go": libSource, "f_test.go": failingTest})
	out := exec.Run(stepNamed(t, "coverage"))
	if out.Kind != pipeline.KindFailed || !strings.Contains(stderr.String(), "error[coverage]") || !strings.Contains(stderr.String(), "deliberate failure") {
		t.Errorf("got %+v, %q", out, stderr)
	}
}

func TestUntidyGoModFailsMod(t *testing.T) {
	exec, stderr := executorFor(t, "")
	project(t, exec, map[string]string{
		"go.mod":     fixtureMod + "\nrequire example.com/dep v0.0.0\n\nreplace example.com/dep => ./dep\n",
		"dep/go.mod": "module example.com/dep\n\ngo 1.26\n",
		"main.go":    "package main\n\nfunc main() {}\n",
	})
	out := exec.Run(stepNamed(t, "mod"))
	if out.Kind != pipeline.KindFailed || !strings.Contains(stderr.String(), "error[mod]: .: go.mod or go.sum is not tidy") {
		t.Errorf("got %+v, %q", out, stderr)
	}
}

func TestTidyModulePassesMod(t *testing.T) {
	exec, stderr := executorFor(t, "")
	project(t, exec, map[string]string{"go.mod": fixtureMod, "main.go": "package main\n\nfunc main() {}\n"})
	if out := exec.Run(stepNamed(t, "mod")); out.Kind != pipeline.KindPassed || stderr.Len() != 0 {
		t.Errorf("got %+v, %q", out, stderr)
	}
}

func TestTestFailureFailsTestWithItsOutput(t *testing.T) {
	exec, stderr := executorFor(t, "")
	project(t, exec, map[string]string{"go.mod": fixtureMod, "f.go": libSource, "f_test.go": failingTest})
	out := exec.Run(stepNamed(t, "test"))
	if out.Kind != pipeline.KindFailed || !strings.Contains(stderr.String(), "error[test]: the tests failed in .") || !strings.Contains(stderr.String(), "deliberate failure") {
		t.Errorf("got %+v, %q", out, stderr)
	}
}

func TestPassingTestsPass(t *testing.T) {
	exec, stderr := executorFor(t, "")
	project(t, exec, map[string]string{"go.mod": fixtureMod, "f.go": libSource, "f_test.go": passingTest})
	if out := exec.Run(stepNamed(t, "test")); out.Kind != pipeline.KindPassed || stderr.Len() != 0 {
		t.Errorf("got %+v, %q", out, stderr)
	}
}

func TestATestFailureStopsLaterRunners(t *testing.T) {
	exec, _ := executorFor(t, "")
	project(t, exec, map[string]string{"go.mod": fixtureMod, "f.go": libSource, "f_test.go": failingTest})
	exec.Ctx.Config.Tools.Record = false
	steps, err := tools.Select([]string{"test", "coverage", "mutants"}, exec.Ctx.Config)
	if err != nil {
		t.Fatal(err)
	}
	report := pipeline.Execute(steps, exec, false)
	if report.Passed || len(report.Results) != 1 || report.Results[0].Step.Name != "test" {
		t.Errorf("got %+v", report)
	}
}

func TestMutantsOffIsSkippedAndSaysSo(t *testing.T) {
	exec, _ := executorFor(t, "")
	out := exec.Run(stepNamed(t, "mutants"))
	if out.Kind != pipeline.KindSkipped || !strings.Contains(out.Reason, "off") {
		t.Errorf("got %+v", out)
	}
}

func TestMutantsDiffRunsGremlinsAgainstTheBase(t *testing.T) {
	exec, stderr := executorFor(t, "")
	exec.Ctx.Config.Mutants.Mode = config.MutantsDiff
	project(t, exec, map[string]string{"go.mod": fixtureMod})
	log := faketool.Install(t, "gremlins", faketool.Spec{})
	if out := exec.Run(stepNamed(t, "mutants")); out.Kind != pipeline.KindPassed || stderr.Len() != 0 {
		t.Fatalf("got %+v, %q", out, stderr)
	}
	calls := faketool.Calls(t, log)
	if len(calls) != 1 || !strings.Contains(calls[0], "\tunleash --config ") || !strings.HasSuffix(calls[0], "/gremlins.yaml --diff origin/main") {
		t.Errorf("calls: %q", calls)
	}
}

func TestMutantsSurvivorsFailWithErrorMutants(t *testing.T) {
	exec, stderr := executorFor(t, "")
	exec.Ctx.Config.Mutants.Mode = config.MutantsFull
	project(t, exec, map[string]string{"go.mod": fixtureMod})
	faketool.Install(t, "gremlins", faketool.Spec{Stdout: "LIVED f.go:3\n", Code: 10})
	out := exec.Run(stepNamed(t, "mutants"))
	if out.Kind != pipeline.KindFailed || !strings.Contains(stderr.String(), "error[mutants]") || !strings.Contains(stderr.String(), "LIVED f.go:3") {
		t.Errorf("got %+v, %q", out, stderr)
	}
}

const vulnStream = `{"osv":{"id":"GO-2000-0001","summary":"Panic in parsing"}}
{"finding":{"osv":"GO-2000-0001","fixed_version":"v1.1.0","trace":[{"module":"example.org/dep","version":"v1.0.0","package":"example.org/dep/parse","function":"Parse"},{"module":"example.com/fixture","package":"example.com/fixture","function":"main","position":{"filename":"main.go","line":5}}]}}
`

func TestVulnReportsAReachedVulnerabilityUnderItsAdvisoryID(t *testing.T) {
	exec, stderr := executorFor(t, "")
	project(t, exec, map[string]string{"go.mod": fixtureMod})
	faketool.Install(t, "govulncheck", faketool.Spec{Stdout: vulnStream})
	out := exec.Run(stepNamed(t, "vuln"))
	if out.Kind != pipeline.KindFailed || !strings.Contains(stderr.String(), "error[GO-2000-0001]: Panic in parsing") || !strings.Contains(stderr.String(), "--> main.go:5") {
		t.Errorf("got %+v, %q", out, stderr)
	}
}

const vulnRecord = `[[exception]]
rule = "GO-2000-0001"
path = "main.go"
covers = "findings"
justification = "the parser is only fed our own checked-in fixtures, never input from outside the process"
`

func TestVulnExceptionLicensesTheAdvisoryAndGoesStaleOnceItIsFixed(t *testing.T) {
	exec, stderr := executorFor(t, vulnRecord)
	project(t, exec, map[string]string{"go.mod": fixtureMod})
	faketool.Install(t, "govulncheck", faketool.Spec{Stdout: vulnStream})
	if out := exec.Run(stepNamed(t, "vuln")); out.Kind != pipeline.KindPassed || stderr.Len() != 0 {
		t.Fatalf("licensed: got %+v, %q", out, stderr)
	}
	faketool.Install(t, "govulncheck", faketool.Spec{})
	out := exec.Run(stepNamed(t, "vuln"))
	if out.Kind != pipeline.KindFailed || !strings.Contains(stderr.String(), "error[gqaci.record.stale]") {
		t.Errorf("fixed advisory: got %+v, %q", out, stderr)
	}
}

func TestDeadcodeFailsAnUnreachableFunctionAndSkipsLibraries(t *testing.T) {
	exec, stderr := executorFor(t, "")
	project(t, exec, map[string]string{"go.mod": fixtureMod, "main.go": mainSource})
	faketool.Install(t, "deadcode", faketool.Spec{Stdout: `[{"Name":"main","Path":"example.com/fixture","Funcs":[{"Name":"unused","Position":{"File":"main.go","Line":5,"Col":6}}]}]`})
	out := exec.Run(stepNamed(t, "deadcode"))
	if out.Kind != pipeline.KindFailed || !strings.Contains(stderr.String(), "error[deadcode]") || !strings.Contains(stderr.String(), "--> main.go:5") {
		t.Fatalf("got %+v, %q", out, stderr)
	}
	library, _ := executorFor(t, "")
	project(t, library, map[string]string{"go.mod": fixtureMod, "f.go": libSource})
	if out := library.Run(stepNamed(t, "deadcode")); out.Kind != pipeline.KindSkipped {
		t.Errorf("library: got %+v", out)
	}
}
