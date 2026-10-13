package tools_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/config"
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
	return tools.Executor{Ctx: tools.Context{Root: root, Config: config.Default()}, Stderr: &stderr}, &stderr
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
