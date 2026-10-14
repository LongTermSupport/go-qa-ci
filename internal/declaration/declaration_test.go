package declaration_test

import (
	"strings"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/declaration"
)

const validDeclaration = `
[specs]
method = "1.1.0"
toolchain = "0.2.0"
detector = "1.0.0"

[project]
conforms = false

[[project.gap]]
clause = "4.1"
summary = "a routed detector does not conform"
record = "docs/example.md"

[artefact]
conforms = false

[[artefact.gap]]
clause = "8.2"
summary = "a bundled tool is off on the own source"
record = "docs/example.md"
`

func TestParseReadsBothLevelsSeparately(t *testing.T) {
	d, err := declaration.Parse([]byte(validDeclaration))
	if err != nil {
		t.Fatal(err)
	}
	if d.Specs.Method != "1.1.0" || d.Specs.Toolchain != "0.2.0" || d.Specs.Detector != "1.0.0" {
		t.Errorf("specs: %+v", d.Specs)
	}
	if len(d.Project.Gaps) != 1 || d.Project.Gaps[0].Clause != "4.1" {
		t.Errorf("project gaps: %+v", d.Project.Gaps)
	}
	if len(d.Artefact.Gaps) != 1 || d.Artefact.Gaps[0].Clause != "8.2" {
		t.Errorf("artefact gaps: %+v", d.Artefact.Gaps)
	}
}

func TestParseRejects(t *testing.T) {
	cases := map[string]struct{ old, new, want string }{
		"an unknown key":               {"[project]", "[project]\nsurprise = 1", "surprise"},
		"a missing spec version":       {`method = "1.1.0"`, "", "specs.method"},
		"a spec version not x.y.z":     {`toolchain = "0.2.0"`, `toolchain = "latest"`, "specs.toolchain"},
		"a claim with a gap":           {"[project]\nconforms = false", "[project]\nconforms = true", "project: conforms = true"},
		"a gap with a bad clause":      {`clause = "4.1"`, `clause = "four"`, "clause"},
		"a gap with no summary":        {`summary = "a routed detector does not conform"`, `summary = " "`, "summary"},
		"a gap that names no record":   {`record = "docs/example.md"` + "\n\n[artefact]", "\n[artefact]", "record"},
		"a section 8 gap in a project": {`clause = "4.1"`, `clause = "8.1"`, "section 8"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			text := strings.Replace(validDeclaration, c.old, c.new, 1)
			if text == validDeclaration {
				t.Fatal("case does not change the declaration")
			}
			_, err := declaration.Parse([]byte(text))
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("want an error naming %q, got %v", c.want, err)
			}
		})
	}
}

func TestParseRejectsADuplicateClauseInOneLevel(t *testing.T) {
	text := validDeclaration + "\n[[artefact.gap]]\nclause = \"8.2\"\nsummary = \"again\"\nrecord = \"docs/example.md\"\n"
	if _, err := declaration.Parse([]byte(text)); err == nil || !strings.Contains(err.Error(), "twice") {
		t.Fatalf("want a duplicate error, got %v", err)
	}
}

func TestParseRequiresAGapWhereConformanceIsDenied(t *testing.T) {
	text := "[specs]\nmethod = \"1.1.0\"\ntoolchain = \"0.2.0\"\ndetector = \"1.0.0\"\n[project]\nconforms = true\n[artefact]\nconforms = false\n"
	if _, err := declaration.Parse([]byte(text)); err == nil || !strings.Contains(err.Error(), "artefact: conforms = false needs a gap") {
		t.Fatalf("a level that denies conformance must say why, got %v", err)
	}
}

const conformancePage = `# Defence Before Fix conformance

Specs: method 1.1.0, detector 1.0.0, toolchain 0.2.0.

## Project level

| Clause | Requirement | Status | Evidence |
|---|---|---|---|
| 4.1 | Detectors conform | Gap | evidence |
| 4.5 | Order | Met | evidence |
| 6.4 | Defaults | Met as test | evidence |

## Artefact level

| Clause | Requirement | Status | Evidence |
|---|---|---|---|
| 4.1 | Detectors conform | Gap | evidence |
| 8.2 | Own source | Partial | evidence |
`

func TestParseConformanceReadsSpecsAndRows(t *testing.T) {
	c, err := declaration.ParseConformance(conformancePage)
	if err != nil {
		t.Fatal(err)
	}
	if c.Specs.Method != "1.1.0" || c.Specs.Detector != "1.0.0" || c.Specs.Toolchain != "0.2.0" {
		t.Errorf("specs: %+v", c.Specs)
	}
	if len(c.Project) != 3 || c.Project[0].Clause != "4.1" || c.Project[0].Status != "Gap" {
		t.Errorf("project rows: %+v", c.Project)
	}
	if len(c.Artefact) != 2 || c.Artefact[1].Status != "Partial" {
		t.Errorf("artefact rows: %+v", c.Artefact)
	}
}

func TestParseConformanceRejects(t *testing.T) {
	cases := map[string]struct{ old, new, want string }{
		"an unknown status":  {"| Met |", "| Mostly |", "status"},
		"a duplicate clause": {"| 4.5 | Order | Met |", "| 4.1 | Order | Met |", "twice"},
		"no specs line":      {"Specs: method 1.1.0, detector 1.0.0, toolchain 0.2.0.", "", "Specs:"},
		"no artefact level":  {"## Artefact level", "## Other", "Artefact level"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			text := strings.Replace(conformancePage, c.old, c.new, 1)
			if _, err := declaration.ParseConformance(text); err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("want an error naming %q, got %v", c.want, err)
			}
		})
	}
}

func TestReconcileReportsEveryDisagreement(t *testing.T) {
	d, err := declaration.Parse([]byte(validDeclaration))
	if err != nil {
		t.Fatal(err)
	}
	c, err := declaration.ParseConformance(conformancePage)
	if err != nil {
		t.Fatal(err)
	}
	problems := declaration.Reconcile(d, c)
	joined := strings.Join(problems, "\n")
	for _, want := range []string{
		"artefact: clause 4.1 is Gap in the page but has no gap in the declaration",
		"project: ", // required clauses the page does not mark at all
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("want a problem containing %q in:\n%s", want, joined)
		}
	}
}

func TestReconcileReportsADeclaredGapThePageCallsMet(t *testing.T) {
	d, err := declaration.Parse([]byte(validDeclaration))
	if err != nil {
		t.Fatal(err)
	}
	c, err := declaration.ParseConformance(strings.Replace(conformancePage, "| 4.1 | Detectors conform | Gap |", "| 4.1 | Detectors conform | Met |", 1))
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(declaration.Reconcile(d, c), "\n")
	if !strings.Contains(joined, "project: gap 4.1 is declared but the page marks it Met") {
		t.Errorf("a declared gap the page calls Met must be reported:\n%s", joined)
	}
}

func TestReconcileReportsDifferentSpecVersions(t *testing.T) {
	d, err := declaration.Parse([]byte(validDeclaration))
	if err != nil {
		t.Fatal(err)
	}
	c, err := declaration.ParseConformance(strings.Replace(conformancePage, "method 1.1.0", "method 1.0.0", 1))
	if err != nil {
		t.Fatal(err)
	}
	if joined := strings.Join(declaration.Reconcile(d, c), "\n"); !strings.Contains(joined, "method") {
		t.Errorf("a page graded against another spec version must be reported:\n%s", joined)
	}
}
