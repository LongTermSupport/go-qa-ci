package tools_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/config"
	"github.com/LongTermSupport/go-qa-ci/internal/date"
	"github.com/LongTermSupport/go-qa-ci/internal/pipeline"
	"github.com/LongTermSupport/go-qa-ci/internal/tools"
)

func executorFor(t *testing.T, recordToml string) (tools.Executor, *bytes.Buffer) {
	t.Helper()
	root := t.TempDir()
	if recordToml != "" {
		path := filepath.Join(root, "qa", "record.toml")
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(recordToml), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var stderr bytes.Buffer
	return tools.Executor{Ctx: tools.Context{Root: root, Config: config.Default(), Today: fixedToday(t)}, Stderr: &stderr}, &stderr
}

func stepNamed(t *testing.T, name string) pipeline.Step {
	t.Helper()
	tool, found := tools.FromName(name)
	if !found {
		t.Fatalf("no tool %q", name)
	}
	return tool.Step(1)
}

func TestExecutorRecordPassesWithoutARecord(t *testing.T) {
	exec, stderr := executorFor(t, "")
	if out := exec.Run(stepNamed(t, "record")); out.Kind != pipeline.KindPassed || stderr.Len() != 0 {
		t.Errorf("got %+v, %q", out, stderr)
	}
}

func TestExecutorRecordFailsOnAGenericJustificationAndPrintsTheFinding(t *testing.T) {
	exec, stderr := executorFor(t, "[[exception]]\nrule = \"errcheck\"\npath = \"a.go\"\njustification = \"legacy\"\n")
	out := exec.Run(stepNamed(t, "record"))
	if out.Kind != pipeline.KindFailed || !strings.Contains(stderr.String(), "error[gqaci.record.invalid]") {
		t.Errorf("got %+v, %q", out, stderr)
	}
}

func TestExecutorRecordFailsOnAMalformedFileWithTheReason(t *testing.T) {
	exec, stderr := executorFor(t, "[[exception]]\nreson = 1\n")
	out := exec.Run(stepNamed(t, "record"))
	if out.Kind != pipeline.KindFailed || !strings.Contains(stderr.String(), "reson") {
		t.Errorf("got %+v, %q", out, stderr)
	}
}

func TestExecutorStillFailsUnbuiltToolsHonestly(t *testing.T) {
	exec, stderr := executorFor(t, "")
	out := exec.Run(stepNamed(t, "test"))
	if out.Kind != pipeline.KindFailed || out.Reason != "not implemented yet" || stderr.Len() != 0 {
		t.Errorf("got %+v, %q", out, stderr)
	}
}

func fixedToday(t *testing.T) date.Date {
	t.Helper()
	d, err := date.Parse("2026-10-07")
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestExecutorRecordReportsAnExpiredEntryAgainstTheContextClock(t *testing.T) {
	exec, stderr := executorFor(t, "[[exception]]\nrule = \"errcheck\"\npath = \"a.go\"\n"+
		"justification = \"Tests assert on panics deliberately; a panic is the failure signal.\"\n"+
		"decided_by = \"alice\"\nadded_by_model = \"human\"\ndecided_on = 2026-06-01\nreview_by = 2026-07-01\n")
	exec.Ctx.Config.Record.Deciders = []string{"alice"}
	out := exec.Run(stepNamed(t, "record"))
	if out.Kind != pipeline.KindFailed || !strings.Contains(stderr.String(), "error[gqaci.record.expired]") {
		t.Errorf("got %+v, %q", out, stderr)
	}
}

const shrunkRecord = "[ratchet]\nhigh_water = 3\ndecided_by = \"alice\"\ndecided_on = 2026-10-07\n" +
	"reason = \"Tests assert on panics deliberately; a panic is the failure signal.\"\n\n" +
	"[[exception]]\nrule = \"errcheck\"\npath = \"a.go\"\n" +
	"justification = \"Tests assert on panics deliberately; a panic is the failure signal.\"\n" +
	"decided_by = \"alice\"\nadded_by_model = \"human\"\ndecided_on = 2026-10-07\nreview_by = 2026-11-07\n"

func TestExecutorRecordInCIFailsUntilTheLoweredCeilingIsCommitted(t *testing.T) {
	exec, stderr := executorFor(t, shrunkRecord)
	exec.Ctx.Config.Record.Deciders = []string{"alice"}
	exec.Ctx.CI = true
	out := exec.Run(stepNamed(t, "record"))
	if out.Kind != pipeline.KindFailed || !strings.Contains(stderr.String(), "error[gqaci.record.ratchet]") {
		t.Errorf("got %+v, %q", out, stderr)
	}
}

func TestExecutorRecordLocallyLowersTheCeilingAndSaysSo(t *testing.T) {
	exec, stderr := executorFor(t, shrunkRecord)
	exec.Ctx.Config.Record.Deciders = []string{"alice"}
	out := exec.Run(stepNamed(t, "record"))
	if out.Kind != pipeline.KindPassed || !strings.Contains(stderr.String(), "lowered [ratchet] high_water to 1") {
		t.Fatalf("got %+v, %q", out, stderr)
	}
	data, err := os.ReadFile(filepath.Join(exec.Ctx.Root, "qa", "record.toml"))
	if err != nil || !strings.Contains(string(data), "high_water = 1") {
		t.Errorf("file not rewritten: %v\n%s", err, data)
	}
}

func TestExecutorDocsPassesOnACleanProject(t *testing.T) {
	exec, stderr := executorFor(t, "")
	if out := exec.Run(stepNamed(t, "docs")); out.Kind != pipeline.KindPassed || stderr.Len() != 0 {
		t.Errorf("got %+v, %q", out, stderr)
	}
}

func TestExecutorDocsFailsOnAnOrphanPageAndPrintsTheFinding(t *testing.T) {
	exec, stderr := executorFor(t, "")
	path := filepath.Join(exec.Ctx.Root, "docs", "defences", "orphaned.rule.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("# orphaned.rule\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	out := exec.Run(stepNamed(t, "docs"))
	if out.Kind != pipeline.KindFailed || !strings.Contains(stderr.String(), "error[gqaci.docs.orphan]") {
		t.Errorf("got %+v, %q", out, stderr)
	}
}
