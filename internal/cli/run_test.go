package cli_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/cli"
	"github.com/LongTermSupport/go-qa-ci/internal/pipeline"
	"github.com/LongTermSupport/go-qa-ci/internal/tools"
)

// recorder is an executor factory that records the context and the steps run, and fails the named steps.
type recorder struct {
	failing []string
	skipped []string
	ctx     tools.Context
	ran     []string
}

func (r *recorder) factory(ctx tools.Context, _ io.Writer) pipeline.Executor {
	r.ctx = ctx
	return r
}

func (r *recorder) Run(step pipeline.Step) pipeline.Outcome {
	r.ran = append(r.ran, step.Name)
	switch {
	case slices.Contains(r.failing, step.Name):
		return pipeline.Failed()
	case slices.Contains(r.skipped, step.Name):
		return pipeline.Skipped("not applicable")
	default:
		return pipeline.Passed()
	}
}

func runWith(rec *recorder, args ...string) (code int, stdout, stderr string) {
	var out, errOut bytes.Buffer
	code = cli.RunWith(args, &out, &errOut, rec.factory)
	return code, out.String(), errOut.String()
}

func TestRunAllPassExitsZeroWithSummaryAndNoPointer(t *testing.T) {
	t.Setenv("CI", "")
	rec := &recorder{skipped: []string{"mutants"}}
	code, out, errOut := runWith(rec, "--root", module(t, ""), "run")
	if code != 0 || errOut != "" {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	for _, want := range []string{"go-qa summary:", "format     fmt          passed", "runners    mutants      skipped (not applicable)"} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Defence Before Fix") {
		t.Errorf("pointer printed on success:\n%s", out)
	}
	if want := tools.Names(); !reflect.DeepEqual(rec.ran, want) {
		t.Errorf("ran %v, want %v", rec.ran, want)
	}
}

func TestRunDetectorFailureLeavesRunnersUnstarted(t *testing.T) {
	rec := &recorder{failing: []string{"suppression"}}
	code, out, _ := runWith(rec, "--root", module(t, ""), "run")
	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	for _, runner := range []string{"vuln", "mod", "test", "coverage", "mutants"} {
		if slices.Contains(rec.ran, runner) {
			t.Errorf("runner %s started after a detector failure", runner)
		}
	}
	if !slices.Contains(rec.ran, "golangci") {
		t.Errorf("later detectors did not run: %v", rec.ran)
	}
	for _, want := range []string{"suppression  FAILED", "runners were not started", "Defence Before Fix", "https://defence-before-fix.github.io/"} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout lacks %q:\n%s", want, out)
		}
	}
}

func TestRunFailFastStopsAtFirstFailure(t *testing.T) {
	rec := &recorder{failing: []string{"suppression"}}
	code, _, _ := runWith(rec, "--root", module(t, ""), "run", "--fail-fast")
	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	if want := []string{"fmt", "record", "suppression"}; !reflect.DeepEqual(rec.ran, want) {
		t.Errorf("ran %v, want %v", rec.ran, want)
	}
}

func TestRunToolSelectionDedupesAndSorts(t *testing.T) {
	rec := &recorder{}
	code, _, _ := runWith(rec, "--root", module(t, "[tools]\nmutants = false\n"), "run", "-t", "test", "-t", "fmt", "-t", "test", "-t", "mutants")
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if want := []string{"fmt", "record", "test", "mutants"}; !reflect.DeepEqual(rec.ran, want) {
		t.Errorf("ran %v, want %v", rec.ran, want)
	}
}

func TestRunDefaultSelectionHonoursConfigSwitches(t *testing.T) {
	rec := &recorder{}
	runWith(rec, "--root", module(t, "[tools]\nmutants = false\nfmt = false\n"), "run")
	if slices.Contains(rec.ran, "mutants") || slices.Contains(rec.ran, "fmt") {
		t.Errorf("disabled tools ran: %v", rec.ran)
	}
}

func TestRunUnknownToolListsKnownNames(t *testing.T) {
	rec := &recorder{}
	code, out, errOut := runWith(rec, "--root", module(t, ""), "run", "-t", "clippy")
	if code != 2 || out != "" {
		t.Errorf("exit %d, stdout %q; want 2 and empty", code, out)
	}
	for _, want := range append([]string{`"clippy"`}, tools.Names()...) {
		if !strings.Contains(errOut, want) {
			t.Errorf("stderr %q lacks %q", errOut, want)
		}
	}
	if len(rec.ran) != 0 {
		t.Errorf("steps ran: %v", rec.ran)
	}
}

func TestRunScopesToOneFileRelativeToTheRoot(t *testing.T) {
	root := module(t, "")
	file := filepath.Join(root, "pkg", "a.go")
	if err := os.MkdirAll(filepath.Dir(file), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("package pkg\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	rec := &recorder{}
	code, out, _ := runWith(rec, "--root", root, "run", "-t", "golangci", "-p", file)
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if want := []string{"pkg/a.go"}; !reflect.DeepEqual(rec.ctx.Scope, want) {
		t.Errorf("scope = %v, want %v", rec.ctx.Scope, want)
	}
	if !strings.Contains(out, "scope: pkg/a.go\n") {
		t.Errorf("summary does not report the scope:\n%s", out)
	}
	if rec.ctx.Root == "" {
		t.Error("context has no root")
	}
}

func TestRunScopeOutsideTheRootIsAUsageError(t *testing.T) {
	rec := &recorder{}
	code, _, errOut := runWith(rec, "--root", module(t, ""), "run", "-p", t.TempDir())
	if code != 2 || !strings.Contains(errOut, "outside") || len(rec.ran) != 0 {
		t.Errorf("exit %d, stderr %q, ran %v", code, errOut, rec.ran)
	}
}

func TestRunCIDetection(t *testing.T) {
	tests := []struct {
		name string
		env  string
		args []string
		want bool
	}{
		{"unset", "", nil, false},
		{"true", "true", nil, true},
		{"one", "1", nil, true},
		{"false", "false", nil, false},
		{"zero", "0", nil, false},
		{"flag", "", []string{"--ci"}, true},
		{"flag wins over false", "false", []string{"--ci"}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("CI", tc.env)
			rec := &recorder{}
			args := append([]string{"--root", module(t, ""), "run", "-t", "fmt"}, tc.args...)
			runWith(rec, args...)
			if rec.ctx.CI != tc.want {
				t.Errorf("CI = %v, want %v", rec.ctx.CI, tc.want)
			}
		})
	}
}

func TestRunNoFix(t *testing.T) {
	rec := &recorder{}
	runWith(rec, "--root", module(t, ""), "run", "-t", "fmt")
	if !rec.ctx.Fix {
		t.Error("fix is off by default")
	}
	runWith(rec, "--root", module(t, ""), "run", "-t", "fmt", "--no-fix")
	if rec.ctx.Fix {
		t.Error("--no-fix ignored")
	}
}

func TestRunRejectsBadUsage(t *testing.T) {
	for _, args := range [][]string{{"run", "extra"}, {"run", "--nope"}, {"run", "-t"}} {
		rec := &recorder{}
		code, _, _ := runWith(rec, append([]string{"--root", module(t, "")}, args...)...)
		if code != 2 || len(rec.ran) != 0 {
			t.Errorf("%v: exit %d, ran %v; want 2 and nothing run", args, code, rec.ran)
		}
	}
}

func TestDefaultExecutorRunsTheSummaryToolAndSaysItIsOffWithoutAFile(t *testing.T) {
	code, out, errOut := run("--root", module(t, ""), "run", "-t", "summary")
	if code != 0 || !strings.Contains(out, "summary      passed") || !strings.Contains(errOut, "summary.file") {
		t.Errorf("exit %d, stdout %q, stderr %q", code, out, errOut)
	}
}
