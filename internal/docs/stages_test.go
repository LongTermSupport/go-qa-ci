package docs_test

import (
	"os"
	"strings"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/config"
	"github.com/LongTermSupport/go-qa-ci/internal/docs"
	"github.com/LongTermSupport/go-qa-ci/internal/faketool"
	"github.com/LongTermSupport/go-qa-ci/internal/finding"
	"github.com/LongTermSupport/go-qa-ci/internal/stage"
)

func TestMain(m *testing.M) {
	if code, served := faketool.Main(); served {
		os.Exit(code)
	}
	os.Exit(m.Run())
}

const rulesJSON = `[{"id":"web-doc","description":"has docs","doc":"# web-doc\n\n## How to fix correctly\n\nUse the helper.\n"},` +
	`{"id":"web-bare","description":"has none"}]`

func webStage() stage.Stage {
	return stage.Stage{
		Name: "web", Phase: "detectors", Says: "Lint the web code", Fix: "Follow the finding's page",
		Command: []string{"web-lint"}, Rules: []string{"web-rules"}, Output: stage.OutputFindings,
	}
}

func configWith(stages []stage.Stage, resolvers ...config.Resolver) config.Config {
	cfg := config.Default()
	cfg.Stages = stages
	cfg.Resolvers = resolvers
	return cfg
}

func TestAStageNameResolvesToTheStagesOwnDescription(t *testing.T) {
	root := t.TempDir()
	doc, found, err := docs.ForConfig(root, configWith([]stage.Stage{webStage()})).Resolve(t.Context(), "web")
	if err != nil || !found || doc.Source != docs.Stage {
		t.Fatalf("found=%v err=%v doc=%+v", found, err, doc)
	}
	for _, want := range []string{"# web\n", "Lint the web code", "## How to fix correctly\n\nFollow the finding's page"} {
		if !strings.Contains(doc.Text, want) {
			t.Errorf("no %q in:\n%s", want, doc.Text)
		}
	}
}

func TestAProjectPageBeatsTheStageDescription(t *testing.T) {
	root := t.TempDir()
	write(t, root, docsDir+"/web.md", page("web"))
	doc, found, err := docs.ForConfig(root, configWith([]stage.Stage{webStage()})).Resolve(t.Context(), "web")
	if err != nil || !found || doc.Source != docs.Project {
		t.Errorf("found=%v err=%v doc=%+v", found, err, doc)
	}
}

func TestARuleTheStagePrintsResolvesToItsDocOnlyWhenItShipsOne(t *testing.T) {
	faketool.Install(t, "web-rules", faketool.Spec{Stdout: rulesJSON})
	resolver := docs.ForConfig(t.TempDir(), configWith([]stage.Stage{webStage()}))
	doc, found, err := resolver.Resolve(t.Context(), "web-doc")
	if err != nil || !found || doc.Source != docs.Stage || !strings.Contains(doc.Text, "Use the helper.") {
		t.Errorf("web-doc: found=%v err=%v doc=%+v", found, err, doc)
	}
	if _, found, err = resolver.Resolve(t.Context(), "web-bare"); err != nil || found {
		t.Errorf("web-bare is dangling without a doc: found=%v err=%v", found, err)
	}
}

func TestTheRulesCommandRunsOnceNoMatterHowManyIdentifiersAreResolved(t *testing.T) {
	log := faketool.Install(t, "web-rules", faketool.Spec{Stdout: rulesJSON})
	resolver := docs.ForConfig(t.TempDir(), configWith([]stage.Stage{webStage()}))
	for _, id := range []string{"web-doc", "web-bare", "other", "web-doc"} {
		if _, _, err := resolver.Resolve(t.Context(), id); err != nil {
			t.Fatal(err)
		}
	}
	if calls := faketool.Calls(t, log); len(calls) != 1 {
		t.Errorf("rules command ran %d times: %q", len(calls), calls)
	}
}

func TestABrokenRulesCommandIsAnErrorNotADanglingIdentifier(t *testing.T) {
	faketool.Install(t, "web-rules", faketool.Spec{Stdout: "garbage"})
	_, found, err := docs.ForConfig(t.TempDir(), configWith([]stage.Stage{webStage()})).Resolve(t.Context(), "anything")
	if err == nil || found {
		t.Errorf("found=%v err=%v", found, err)
	}
}

func TestAConfiguredResolverDocumentsMatchingIdentifiersLast(t *testing.T) {
	cfg := configWith(nil, config.Resolver{Matches: "SC[0-9]*", Text: "Read the wiki page for {id}."})
	resolver := docs.ForConfig(t.TempDir(), cfg)
	doc, found, err := resolver.Resolve(t.Context(), "SC2086")
	if err != nil || !found || doc.Source != docs.Configured || doc.Text != "Read the wiki page for SC2086." {
		t.Errorf("found=%v err=%v doc=%+v", found, err, doc)
	}
	if _, found, err = resolver.Resolve(t.Context(), "XY1"); err != nil || found {
		t.Errorf("a non-matching identifier resolved: found=%v err=%v", found, err)
	}
}

func TestCheckRequiresPagesForStageNamesAndTheRulesTheyPrint(t *testing.T) {
	faketool.Install(t, "web-rules", faketool.Spec{Stdout: rulesJSON})
	root := t.TempDir()
	findings, err := docs.Run(t.Context(), root, configWith([]stage.Stage{webStage()}))
	if err != nil {
		t.Fatal(err)
	}
	dangling := map[string]bool{}
	for _, f := range findings {
		if f.Rule == finding.DocsDangling && strings.HasPrefix(f.Path, docsDir) {
			dangling[f.Path] = true
		}
	}
	if len(dangling) != 1 || !dangling[docsDir+"/web-bare.md"] {
		t.Errorf("dangling = %v, want only the rule with no doc (the stage describes itself)", dangling)
	}
}

func TestPagesForStageNamesAndRulesAreNotOrphans(t *testing.T) {
	faketool.Install(t, "web-rules", faketool.Spec{Stdout: rulesJSON})
	root := t.TempDir()
	write(t, root, docsDir+"/web.md", page("web"))
	write(t, root, docsDir+"/web-bare.md", page("web-bare"))
	write(t, root, docsDir+"/stray.md", page("stray"))
	findings, err := docs.Run(t.Context(), root, configWith([]stage.Stage{webStage()}))
	if err != nil {
		t.Fatal(err)
	}
	var orphans []string
	for _, f := range findings {
		if f.Rule == finding.DocsOrphan {
			orphans = append(orphans, f.Path)
		}
	}
	if len(orphans) != 1 || orphans[0] != docsDir+"/stray.md" {
		t.Errorf("findings = %v", findings)
	}
}

func TestAConfiguredResolverSatisfiesTheCheckForARuleItMatches(t *testing.T) {
	faketool.Install(t, "web-rules", faketool.Spec{Stdout: `[{"id":"SC2086","description":"quote it"}]`})
	cfg := configWith([]stage.Stage{webStage()}, config.Resolver{Matches: "SC*", Text: "See {id}."})
	findings, err := docs.Run(t.Context(), t.TempDir(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range findings {
		if f.Rule == finding.DocsDangling && strings.HasPrefix(f.Path, docsDir) {
			t.Errorf("dangling: %v", f)
		}
	}
}
