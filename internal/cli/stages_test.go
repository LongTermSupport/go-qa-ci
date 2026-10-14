package cli_test

import (
	"strings"
	"testing"
)

// stageConfig declares one project stage in the given phase.
func stageConfig(name, phase string) string {
	return "[[stage]]\nname = \"" + name + "\"\nphase = \"" + phase + "\"\nsays = \"Check the examples\"\n" +
		"fix = \"Follow the finding\"\ncommand = [\"example-check\"]\n"
}

func TestToolsListsProjectStagesAfterTheCatalogue(t *testing.T) {
	root := module(t, stageConfig("examples", "detectors")+"\n"+stageConfig("e2e", "runners"))
	code, out, errOut := run("--root", root, "tools")
	if code != 0 || errOut != "" {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	if !strings.HasPrefix(out, defaultTools) || !strings.HasSuffix(out, "examples     detectors  stage\ne2e          runners    stage\n") {
		t.Errorf("tools output:\n%s", out)
	}
}

func TestAStageNamedLikeABuiltInToolIsAConfigurationError(t *testing.T) {
	for _, subcommand := range []string{"tools", "run", "rules", "docs-check"} {
		root := module(t, stageConfig("golangci", "detectors"))
		code, out, errOut := run("--root", root, subcommand)
		if code != 2 || out != "" || !strings.Contains(errOut, "stage `golangci` has the name of a built-in tool") {
			t.Errorf("%s: exit %d, stdout %q, stderr %q", subcommand, code, out, errOut)
		}
	}
}

func TestAnInvalidStageIsAConfigurationErrorNamingTheStage(t *testing.T) {
	root := module(t, stageConfig("examples", "sometime"))
	code, _, errOut := run("--root", root, "tools")
	if code != 2 || !strings.Contains(errOut, "stage `examples`") || !strings.Contains(errOut, "unknown phase") {
		t.Errorf("exit %d, stderr %q", code, errOut)
	}
}

func TestRuleDocResolvesAStageNameAndAConfiguredResolver(t *testing.T) {
	root := module(t, stageConfig("examples", "detectors")+"[[resolver]]\nmatches = \"EX[0-9]*\"\ntext = \"Documentation of {id}.\"\n")
	code, out, errOut := run("--root", root, "rule-doc", "examples")
	if code != 0 || !strings.HasPrefix(out, "# examples\n") || !strings.Contains(errOut, "(source: stage)") {
		t.Errorf("stage: exit %d, stdout %q, stderr %q", code, out, errOut)
	}
	code, out, errOut = run("--root", root, "rule-doc", "EX42")
	if code != 0 || strings.TrimSpace(out) != "Documentation of EX42." || !strings.Contains(errOut, "(source: configured resolver)") {
		t.Errorf("resolver: exit %d, stdout %q, stderr %q", code, out, errOut)
	}
}

func TestRunSelectsAStageByName(t *testing.T) {
	root := module(t, "[[stage]]\nname = \"examples\"\nphase = \"detectors\"\nsays = \"s\"\nfix = \"f\"\n"+
		"command = [\"example-check\", \"{files}\"]\nfiles = [\"*.nothing\"]\n")
	code, out, errOut := run("--root", root, "run", "-t", "examples")
	if code != 0 || !strings.Contains(out, "examples") || !strings.Contains(out, "skipped (no files for this stage)") {
		t.Errorf("exit %d, stdout %q, stderr %q", code, out, errOut)
	}
}
