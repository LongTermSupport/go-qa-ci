package runners_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/faketool"
	"github.com/LongTermSupport/go-qa-ci/internal/finding"
	"github.com/LongTermSupport/go-qa-ci/internal/runners"
)

const deadJSON = `[{"Name":"main","Path":"example.com/fixture","Funcs":[
{"Name":"unused","Position":{"File":"main.go","Line":5,"Col":6},"Generated":false,"Marker":false},
{"Name":"gen","Position":{"File":"gen.go","Line":1,"Col":6},"Generated":true,"Marker":false},
{"Name":"T.marker","Position":{"File":"m.go","Line":2,"Col":6},"Generated":false,"Marker":true}]}]`

func TestParseDeadcodeSkipsGeneratedAndMarkerFunctions(t *testing.T) {
	got, err := runners.ParseDeadcode([]byte(deadJSON))
	if err != nil {
		t.Fatal(err)
	}
	want := finding.Finding{Rule: "deadcode", Path: "main.go", Line: 5, Message: "example.com/fixture.unused is unreachable from every main function and test"}
	if len(got) != 1 || got[0] != want {
		t.Errorf("got %+v", got)
	}
}

func TestParseDeadcodeReadsNullAsNothingDead(t *testing.T) {
	got, err := runners.ParseDeadcode([]byte("null"))
	if err != nil || len(got) != 0 {
		t.Errorf("got %v, %v", got, err)
	}
	if _, err := runners.ParseDeadcode([]byte("{")); err == nil {
		t.Error("malformed output accepted")
	}
}

func deadcodeCall(t *testing.T) error {
	t.Helper()
	if _, _, err := runners.Deadcode(ctx(), module(t, map[string]string{"main.go": "package main\n\nfunc main() {}\n"})); err != nil {
		return fmt.Errorf("deadcode: %w", err)
	}
	return nil
}

func TestDeadcodeAnalysesAModuleWithAMainPackage(t *testing.T) {
	log := faketool.Install(t, "deadcode", faketool.Spec{Stdout: deadJSON})
	found, applies, err := runners.Deadcode(ctx(), module(t, map[string]string{"main.go": "package main\n\nfunc main() {}\n"}))
	if err != nil || !applies || len(found) != 1 {
		t.Fatalf("got %v, %v, %v", found, applies, err)
	}
	if calls := faketool.Calls(t, log); len(calls) != 1 || !strings.HasSuffix(calls[0], "\t-test -json ./...") {
		t.Errorf("calls: %q", calls)
	}
}

func TestDeadcodeDoesNotApplyToALibrary(t *testing.T) {
	log := faketool.Install(t, "deadcode", faketool.Spec{Stdout: deadJSON})
	found, applies, err := runners.Deadcode(ctx(), module(t, map[string]string{"lib.go": "package fixture\n\nfunc Exported() {}\n"}))
	if err != nil || applies || len(found) != 0 {
		t.Errorf("got %v, %v, %v", found, applies, err)
	}
	if calls := faketool.Calls(t, log); len(calls) != 0 {
		t.Errorf("deadcode ran on a library: %q", calls)
	}
}

func TestDeadcodeFailsWhenTheAnalysisFails(t *testing.T) {
	faketool.Install(t, "deadcode", faketool.Spec{Stderr: "load error\n", Code: 1})
	if err := deadcodeCall(t); err == nil || !strings.Contains(err.Error(), "load error") {
		t.Errorf("got %v", err)
	}
}
