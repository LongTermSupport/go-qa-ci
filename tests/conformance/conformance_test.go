// Package conformance_test keeps the machine-readable declaration, the clause-by-clause page and
// the README telling the same story (TOOLING-SPEC 9, 9.1, 9.2).
package conformance_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/declaration"
)

func readRepoFile(t *testing.T, path string) string {
	t.Helper()
	root, err := os.OpenRoot(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := root.Close(); closeErr != nil {
			t.Error(closeErr)
		}
	}()
	data, err := root.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func load(t *testing.T) (declaration.Declaration, declaration.Conformance) {
	t.Helper()
	d, err := declaration.Parse([]byte(readRepoFile(t, "defence-before-fix.toml")))
	if err != nil {
		t.Fatal(err)
	}
	c, err := declaration.ParseConformance(readRepoFile(t, "docs/CONFORMANCE.md"))
	if err != nil {
		t.Fatal(err)
	}
	return d, c
}

func TestDeclarationListsAGapForEveryGapOrPartialClauseOfThePage(t *testing.T) {
	d, c := load(t)
	for _, problem := range declaration.Reconcile(d, c) {
		t.Error(problem)
	}
}

func TestEveryRecordedGapPointsAtAFileThatExists(t *testing.T) {
	d, _ := load(t)
	for _, g := range slices.Concat(d.Project.Gaps, d.Artefact.Gaps) {
		if readRepoFile(t, g.Record) == "" {
			t.Errorf("gap %s names an empty record file %s", g.Clause, g.Record)
		}
	}
}

func TestNoLevelClaimsConformanceWhileAGapIsRecorded(t *testing.T) {
	d, _ := load(t)
	if d.Project.Conforms && len(d.Project.Gaps) > 0 || d.Artefact.Conforms && len(d.Artefact.Gaps) > 0 {
		t.Fatal("a level claims conformance with a gap recorded (TOOLING-SPEC 9)")
	}
}

func TestREADMEStatesTheStandingWithoutStaleOrOverclaimingWording(t *testing.T) {
	readme := readRepoFile(t, "README.md")
	for _, stale := range []string{"No declaration exists yet", "no gap list", "Work in progress: no conformance"} {
		if strings.Contains(readme, stale) {
			t.Errorf("README carries stale wording %q", stale)
		}
	}
	for _, want := range []string{"defence-before-fix.toml", "docs/CONFORMANCE.md", "does not claim conformance"} {
		if !strings.Contains(readme, want) {
			t.Errorf("README must mention %q", want)
		}
	}
}

func TestREADMESectionsFollowTheSiblingOrder(t *testing.T) {
	readme := readRepoFile(t, "README.md")
	last := -1
	for _, heading := range []string{
		"# go-qa-ci", "## What it runs", "## Writing rules", "## Project stages", "## Project record",
		"## Configuration", "## Installing", "## Public-safety guard",
	} {
		at := strings.Index(readme, "\n"+heading+"\n")
		if heading == "# go-qa-ci" {
			at = strings.Index(readme, heading+"\n")
		}
		if at < 0 {
			t.Errorf("README has no %q heading", heading)
			continue
		}
		if at < last {
			t.Errorf("README heading %q is out of order", heading)
		}
		last = at
	}
}
