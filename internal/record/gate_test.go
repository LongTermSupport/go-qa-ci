package record_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/date"
	"github.com/LongTermSupport/go-qa-ci/internal/finding"
	"github.com/LongTermSupport/go-qa-ci/internal/record"
)

const recordRel = "qa/record.toml"

func recordText(t *testing.T, highWater int, signedOn date.Date, paths ...string) string {
	t.Helper()
	var b strings.Builder
	fmt.Fprintf(&b, "[ratchet]\nhigh_water = %d\ndecided_by = %q\ndecided_on = %s\nreason = %q\n",
		highWater, decider, signedOn, soundReason)
	now := today(t)
	for _, p := range paths {
		fmt.Fprintf(&b, "\n[[exception]]\nrule = \"errcheck\"\npath = %q\njustification = %q\ndecided_by = %q\n"+
			"added_by_model = \"human\"\ndecided_on = %s\nreview_by = %s\n", p, soundReason, decider, now, now.AddDays(60))
	}
	return b.String()
}

func git(t *testing.T, root string, args ...string) {
	t.Helper()
	full := append([]string{"-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false"}, args...)
	cmd := exec.CommandContext(t.Context(), "sh", "-c", `exec git "$@"`, "sh")
	cmd.Args = append(cmd.Args, full...)
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func writeRecord(t *testing.T, root, text string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(recordRel))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

func commitRecord(t *testing.T, root, text string) {
	t.Helper()
	writeRecord(t, root, text)
	git(t, root, "add", ".")
	git(t, root, "commit", "-q", "-m", "record")
}

// repoWith returns a git repository whose main branch commits the given record.
func repoWith(t *testing.T, text string) string {
	t.Helper()
	root := t.TempDir()
	git(t, root, "init", "-q", "-b", "main")
	commitRecord(t, root, text)
	return root
}

func gate(t *testing.T, root string, mayLower bool) record.GateReport {
	t.Helper()
	policy := testPolicy()
	policy.Path = recordRel
	policy.HistoryBase = "main"
	report, err := record.Gate(root, policy, today(t), mayLower)
	if err != nil {
		t.Fatal(err)
	}
	return report
}

func ratchetFindings(report record.GateReport) []finding.Finding {
	var out []finding.Finding
	for _, f := range report.Findings {
		if f.Rule == finding.RecordRatchet {
			out = append(out, f)
		}
	}
	return out
}

func TestAnUnsignedRaiseAgainstHeadFails(t *testing.T) {
	signedOn := today(t).AddDays(-3)
	root := repoWith(t, recordText(t, 1, signedOn, "a.go"))
	writeRecord(t, root, recordText(t, 2, signedOn, "a.go", "b.go"))
	assertOne(t, ratchetFindings(gate(t, root, false)), finding.RecordRatchet, "raised from 1 to 2")
}

func TestAStaleSignatureOnARaiseFails(t *testing.T) {
	root := repoWith(t, recordText(t, 1, today(t).AddDays(-60), "a.go"))
	writeRecord(t, root, recordText(t, 2, today(t).AddDays(-30), "a.go", "b.go"))
	assertOne(t, ratchetFindings(gate(t, root, false)), finding.RecordRatchet, "14 days")
}

func TestAFreshSignatureOnARaisePasses(t *testing.T) {
	root := repoWith(t, recordText(t, 1, today(t).AddDays(-60), "a.go"))
	writeRecord(t, root, recordText(t, 2, today(t), "a.go", "b.go"))
	if got := gate(t, root, false); len(got.Findings) != 0 {
		t.Errorf("got %v", got.Findings)
	}
}

func TestACommittedRaiseIsStillCaughtAgainstTheMergeBase(t *testing.T) {
	signedOn := today(t).AddDays(-3)
	root := repoWith(t, recordText(t, 1, signedOn, "a.go"))
	git(t, root, "checkout", "-q", "-b", "feature")
	commitRecord(t, root, recordText(t, 2, signedOn, "a.go", "b.go"))
	assertOne(t, ratchetFindings(gate(t, root, false)), finding.RecordRatchet, "raised from 1 to 2")
}

func TestARaiseBeforeTheBaseWasFetchedUsesHead(t *testing.T) {
	root := repoWith(t, recordText(t, 1, today(t).AddDays(-3), "a.go"))
	writeRecord(t, root, recordText(t, 2, today(t).AddDays(-3), "a.go", "b.go"))
	policy := testPolicy()
	policy.Path = recordRel
	policy.HistoryBase = "origin/no-such-ref"
	report, err := record.Gate(root, policy, today(t), false)
	if err != nil {
		t.Fatal(err)
	}
	found := ratchetFindings(report)
	if len(found) != 2 {
		t.Fatalf("want the raise and the unresolved base, got %v", found)
	}
	joined := found[0].Message + found[1].Message
	if !strings.Contains(joined, "raised from 1 to 2") || !strings.Contains(joined, "does not resolve") {
		t.Errorf("got %v", found)
	}
}

func TestACommittedRaiseWithAnUnresolvableBaseIsAGapNotAPass(t *testing.T) {
	signedOn := today(t).AddDays(-3)
	root := repoWith(t, recordText(t, 1, signedOn, "a.go"))
	git(t, root, "checkout", "-q", "-b", "feature")
	commitRecord(t, root, recordText(t, 2, signedOn, "a.go", "b.go"))
	policy := testPolicy()
	policy.Path = recordRel
	policy.HistoryBase = "origin/main"

	ci, err := record.Gate(root, policy, today(t), false)
	if err != nil {
		t.Fatal(err)
	}
	assertOne(t, ratchetFindings(ci), finding.RecordRatchet, `history_base "origin/main" does not resolve`)

	local, err := record.Gate(root, policy, today(t), true)
	if err != nil {
		t.Fatal(err)
	}
	if len(local.Findings) != 0 || len(local.Notes) != 1 || !strings.Contains(local.Notes[0], "does not resolve") {
		t.Errorf("got %v, %v", local.Findings, local.Notes)
	}
}

func TestWithoutHistoryTheGateSaysSo(t *testing.T) {
	root := t.TempDir()
	writeRecord(t, root, recordText(t, 1, today(t), "a.go"))
	report := gate(t, root, false)
	if len(report.Findings) != 0 || len(report.Notes) != 1 || !strings.Contains(report.Notes[0], "no committed history") {
		t.Errorf("got %v, %v", report.Findings, report.Notes)
	}
}

func TestAShrunkRecordMustLowerItsCeiling(t *testing.T) {
	root := repoWith(t, recordText(t, 2, today(t), "a.go", "b.go"))
	writeRecord(t, root, recordText(t, 2, today(t), "a.go"))
	assertOne(t, ratchetFindings(gate(t, root, false)), finding.RecordRatchet, "lower it to 1")
}

func TestAShrunkRecordHasItsCeilingLoweredWhenAllowed(t *testing.T) {
	root := repoWith(t, recordText(t, 2, today(t), "a.go", "b.go"))
	writeRecord(t, root, recordText(t, 2, today(t), "a.go"))
	report := gate(t, root, true)
	if len(report.Findings) != 0 || len(report.Notes) != 1 || !strings.Contains(report.Notes[0], "lowered") {
		t.Fatalf("got %v, %v", report.Findings, report.Notes)
	}
	if report.Record.Ratchet.HighWater != 1 {
		t.Errorf("record still at %d", report.Record.Ratchet.HighWater)
	}
	rec, err := record.Load(filepath.Join(root, filepath.FromSlash(recordRel)))
	if err != nil || rec.Ratchet.HighWater != 1 {
		t.Errorf("file not rewritten: %+v, %v", rec.Ratchet, err)
	}
}

func TestPruneRemovesEntriesAndLowersHighWater(t *testing.T) {
	root := repoWith(t, recordText(t, 3, today(t), "a.go", "b.go", "c.go"))
	remaining, err := record.Prune(root, recordRel, []int{0, 2})
	if err != nil || remaining != 1 {
		t.Fatalf("got %d, %v", remaining, err)
	}
	rec, err := record.Load(filepath.Join(root, filepath.FromSlash(recordRel)))
	if err != nil {
		t.Fatal(err)
	}
	if rec.Ratchet.HighWater != 1 || len(rec.Exceptions) != 1 || rec.Exceptions[0].Path != "b.go" {
		t.Errorf("got %+v", rec)
	}
	// The pruned record is a shrink, not a raise.
	if got := gate(t, root, false).Findings; len(got) != 0 {
		t.Errorf("pruned record fails the gate: %v", got)
	}
}

func TestPruneOfARecordWithoutARatchetOnlyRemoves(t *testing.T) {
	root := t.TempDir()
	text := strings.SplitN(recordText(t, 2, today(t), "a.go", "b.go"), "\n[[exception]]", 2)
	writeRecord(t, root, "[[exception]]"+text[1])
	remaining, err := record.Prune(root, recordRel, []int{0})
	if err != nil || remaining != 1 {
		t.Errorf("got %d, %v", remaining, err)
	}
}

func TestPruneRejectsAnEntryNumberThatDoesNotExist(t *testing.T) {
	root := repoWith(t, recordText(t, 1, today(t), "a.go"))
	if _, err := record.Prune(root, recordRel, []int{4}); err == nil {
		t.Error("out-of-range index accepted")
	}
}
