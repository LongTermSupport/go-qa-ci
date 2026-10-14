package stage_test

import (
	"reflect"
	"strings"
	"testing"

	toml "github.com/pelletier/go-toml/v2"

	"github.com/LongTermSupport/go-qa-ci/internal/pipeline"
	"github.com/LongTermSupport/go-qa-ci/internal/stage"
)

// The globs are assembled from parts so that no source line holds a comment opener.
const (
	anyShell  = "**/" + "*.sh"
	anyTS     = "**/" + "*.ts"
	vendorAll = "vendor/" + "**"
)

// shell is a ShellCheck-style stage: whole run over every shell script.
const shell = `
name = "shellcheck"
phase = "detectors"
says = "ShellCheck over every shell script"
fix = "Quote expansions and check every status, as the finding's page says"
command = ["shellcheck", "{files}"]
files = ["` + anyShell + `"]
shebangs = ["bash", "sh"]
exclude = ["` + vendorAll + `"]
`

func parse(t *testing.T, text string) stage.Stage {
	t.Helper()
	var s stage.Stage
	if err := toml.Unmarshal([]byte(text), &s); err != nil {
		t.Fatal(err)
	}
	return s
}

// argv is the command line of a run that must not be skipped.
func argv(t *testing.T, s stage.Stage, scoped bool, files []string) []string {
	t.Helper()
	got, ok := s.Argv(scoped, files)
	if !ok {
		t.Fatalf("%s skipped with files %v", s.Name, files)
	}
	return got
}

func TestAStageReadsFromTheConfiguration(t *testing.T) {
	s := parse(t, shell)
	phase, err := s.PhaseOf()
	if s.Name != "shellcheck" || err != nil || phase != pipeline.PhaseDetectors || s.Output.Effective() != stage.OutputStatus || s.Dir != "" {
		t.Errorf("stage = %+v, phase %v, %v", s, phase, err)
	}
	if problems := s.Problems(); len(problems) != 0 {
		t.Errorf("problems: %v", problems)
	}
}

func TestAStageMustSayWhatItChecksHowToFixItAndWhatItRuns(t *testing.T) {
	s := parse(t, "name = \"\"\nphase = \"runners\"\nsays = \" \"\nfix = \"\"\ncommand = []\n")
	problems := strings.Join(s.Problems(), "\n")
	for _, want := range []string{"no name", "`says`", "`fix`", "empty command"} {
		if !strings.Contains(problems, want) {
			t.Errorf("no %q in:\n%s", want, problems)
		}
	}
}

func TestAStageNeedsAKnownPhaseAndOutput(t *testing.T) {
	s := parse(t, "name = \"x\"\nphase = \"later\"\nsays = \"s\"\nfix = \"f\"\ncommand = [\"x\"]\noutput = \"json\"\n")
	problems := strings.Join(s.Problems(), "\n")
	for _, want := range []string{"unknown phase", "format, detectors or runners", "output \"json\"", "status or findings"} {
		if !strings.Contains(problems, want) {
			t.Errorf("no %q in:\n%s", want, problems)
		}
	}
	missing := parse(t, "name = \"x\"\nsays = \"s\"\nfix = \"f\"\ncommand = [\"x\"]\n")
	if joined := strings.Join(missing.Problems(), "\n"); !strings.Contains(joined, "unknown phase") {
		t.Errorf("a stage with no phase passed: %s", joined)
	}
}

func TestANameMustBeUsableAsAnIdentifierAndOutsideTheToolchainNamespace(t *testing.T) {
	for name, want := range map[string]string{
		"two words":     "identifier",
		"a/b":           "identifier",
		"gqaci.mine":    "gqaci.",
		"fine-name_1.2": "",
	} {
		s := parse(t, "name = \""+name+"\"\nphase = \"runners\"\nsays = \"s\"\nfix = \"f\"\ncommand = [\"x\"]\n")
		problems := strings.Join(s.Problems(), "\n")
		if (want == "") != (problems == "") || !strings.Contains(problems, want) {
			t.Errorf("name %q: problems %q, want %q", name, problems, want)
		}
	}
}

func TestFilePlaceholdersNeedAFileSelectionAndAWholeArgument(t *testing.T) {
	s := parse(t, "name = \"x\"\nphase = \"detectors\"\nsays = \"s\"\nfix = \"f\"\n"+
		"command = [\"tool\", \"--files={files}\"]\nfiles_command = [\"tool\"]\n")
	problems := strings.Join(s.Problems(), "\n")
	if !strings.Contains(problems, "whole argument") || !strings.Contains(problems, "selects no files") {
		t.Errorf("problems:\n%s", problems)
	}
}

func TestGlobsAndDirectoryAreChecked(t *testing.T) {
	s := parse(t, "name = \"x\"\nphase = \"detectors\"\nsays = \"s\"\nfix = \"f\"\ncommand = [\"tool\"]\n"+
		"files = [\"[\"]\nexclude = [\"a[\"]\ndir = \"../out\"\n")
	problems := strings.Join(s.Problems(), "\n")
	if strings.Count(problems, "invalid glob") != 2 || !strings.Contains(problems, "dir \"../out\"") {
		t.Errorf("problems:\n%s", problems)
	}
}

func TestTheFilesPlaceholderExpandsToOneArgumentPerFile(t *testing.T) {
	s := parse(t, shell)
	got := argv(t, s, false, []string{"a.sh", "b c.sh"})
	if want := []string{"shellcheck", "a.sh", "b c.sh"}; !reflect.DeepEqual(got, want) {
		t.Errorf("argv = %v; want %v", got, want)
	}
	if _, ok := s.Argv(false, nil); ok {
		t.Error("nothing to check must be a skip, not a run with no files")
	}
}

func TestASubsetRunUsesTheFilesCommandAndAStageWithoutOneRunsWhole(t *testing.T) {
	formatter := parse(t, "name = \"prettier\"\nphase = \"format\"\nsays = \"s\"\nfix = \"f\"\n"+
		"command = [\"prettier\", \"--check\", \".\"]\nfiles_command = [\"prettier\", \"--check\", \"{files}\"]\nfiles = [\""+anyTS+"\"]\n")
	files := []string{"src/a.ts"}
	if got := strings.Join(argv(t, formatter, false, files), " "); got != "prettier --check ." {
		t.Errorf("whole run: %s", got)
	}
	if got := strings.Join(argv(t, formatter, true, files), " "); got != "prettier --check src/a.ts" {
		t.Errorf("subset run: %s", got)
	}
	plain := parse(t, "name = \"tsc\"\nphase = \"detectors\"\nsays = \"s\"\nfix = \"f\"\ncommand = [\"tsc\", \"--noEmit\"]\n")
	if got := strings.Join(argv(t, plain, true, files), " "); got != "tsc --noEmit" {
		t.Errorf("a stage with no files_command runs whole: %s", got)
	}
	if formatter.TakesFiles(false) || !formatter.TakesFiles(true) || plain.TakesFiles(true) {
		t.Error("TakesFiles must follow the command line chosen for the run")
	}
}

func TestFilesArePassedRelativeToTheStageDirectory(t *testing.T) {
	s := parse(t, "name = \"x\"\nphase = \"detectors\"\nsays = \"s\"\nfix = \"f\"\ndir = \"web\"\n"+
		"command = [\"lint\", \"{files}\"]\nfiles = [\"web/"+anyTS+"\"]\n")
	got := argv(t, s, false, []string{"web/src/a.ts", "shared/b.ts"})
	if want := []string{"lint", "src/a.ts", "../shared/b.ts"}; !reflect.DeepEqual(got, want) {
		t.Errorf("argv = %v; want %v", got, want)
	}
}

func TestAStageDocumentsItselfWithTheCorrectConstruction(t *testing.T) {
	doc := parse(t, shell).Doc()
	for _, want := range []string{
		"# shellcheck\n", "ShellCheck over every shell script",
		"\n## How to fix correctly\n\nQuote expansions", "`shellcheck {files}`",
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("no %q in:\n%s", want, doc)
		}
	}
	withDir := parse(t, "name = \"x\"\nphase = \"runners\"\nsays = \"s\"\nfix = \"f\"\ncommand = [\"t\"]\ndir = \"web\"\n").Doc()
	if !strings.Contains(withDir, "in `web`") {
		t.Errorf("the directory is missing:\n%s", withDir)
	}
}
