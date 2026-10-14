package record

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/LongTermSupport/go-qa-ci/internal/config"
	"github.com/LongTermSupport/go-qa-ci/internal/date"
	"github.com/LongTermSupport/go-qa-ci/internal/finding"
)

// GateReport is the outcome of the record gate.
type GateReport struct {
	// Record is the record after any lowering of its ceiling.
	Record Record
	// Findings are the structure, policy and ratchet violations.
	Findings []finding.Finding
	// Notes are informational lines: a lowered ceiling, unavailable history.
	Notes []string
}

// Gate runs every record control for the project at root: structure, policy and the ratchet
// compared with the record as committed at HEAD and at the merge base with policy.HistoryBase.
// When mayLower is set and the record shrank, high_water is rewritten to the new count; otherwise
// the unlowered ceiling is a finding, so CI fails until the lowered ceiling is committed.
func Gate(root string, policy config.RecordPolicy, today date.Date, mayLower bool) (GateReport, error) {
	file := filepath.Join(root, filepath.FromSlash(policy.Path))
	rec, err := Load(file)
	if err != nil {
		return GateReport{}, fmt.Errorf("%s: %w", policy.Path, err)
	}
	report := GateReport{Record: rec, Findings: rec.Validate(policy.Path)}
	in := Input{Record: rec, Path: policy.Path, Policy: policy, Today: today}
	report.Findings = append(report.Findings, Review(in)...)

	previous, baseUnresolved := previousRecord(root, policy.Path, policy.HistoryBase)
	// A raise can be seen only against history, so history that is not there is a gap, not a pass:
	// it fails where the record is enforced (CI) and is noted locally.
	var gap string
	switch {
	case len(rec.Exceptions) == 0:
	case previous == nil && headResolves(root):
		gap = "the record committed at HEAD does not parse, so a raise of high_water is not checked against it"
	case previous == nil:
		gap = "no committed history for the record (no repository, no commit, or no git); a raise of high_water is not checked against it"
	case baseUnresolved:
		gap = fmt.Sprintf("history_base %q does not resolve to a merge base with HEAD (not fetched, no such ref, or a shallow clone); "+
			"a committed raise of high_water is not checked against it", policy.HistoryBase)
	}
	switch {
	case gap == "":
	case mayLower:
		report.Notes = append(report.Notes, gap)
	default:
		report.Findings = append(report.Findings, in.ratchetFinding("[ratchet] "+gap))
	}
	verdict := ReviewRatchet(in, previous)
	report.Findings = append(report.Findings, verdict.Findings...)
	if verdict.LowerTo == nil {
		return report, nil
	}
	count := *verdict.LowerTo
	if !mayLower {
		report.Findings = append(report.Findings, in.ratchetFinding(fmt.Sprintf(
			"[ratchet] high_water %d is above the %d exception(s) the record holds; lower it to %d (a local run does this automatically)",
			Ceiling(rec), count, count)))
		return report, nil
	}
	if err := rewriteHighWater(file, count); err != nil {
		return GateReport{}, fmt.Errorf("%s: %w", policy.Path, err)
	}
	report.Record.Ratchet.HighWater = count
	report.Notes = append(report.Notes, fmt.Sprintf("the record shrank: lowered [ratchet] high_water to %d; commit the change", count))
	return report, nil
}

var errNoHighWater = errors.New("no high_water line in the [ratchet] table to lower")

func rewriteHighWater(file string, value int) error {
	data, err := readFile(file)
	if err != nil {
		return err
	}
	text, ok := SetHighWater(string(data), value)
	if !ok {
		return errNoHighWater
	}
	return writeFile(file, text)
}

// Prune removes the exceptions at indexes (0-based, file order, a repeated index counts once) from
// the record at recordPath under root, and lowers high_water to the count that remains. A record
// without a [ratchet] table only loses the entries. It returns the number of exceptions removed
// and the number that remain.
func Prune(root, recordPath string, indexes []int) (removed, remaining int, err error) {
	file := filepath.Join(root, filepath.FromSlash(recordPath))
	data, err := readFile(file)
	if err != nil {
		return 0, 0, fmt.Errorf("%s: %w", recordPath, err)
	}
	rec, err := Parse(data)
	if err != nil {
		return 0, 0, fmt.Errorf("%s: %w", recordPath, err)
	}
	for _, index := range indexes {
		if index < 0 || index >= len(rec.Exceptions) {
			return 0, 0, fmt.Errorf("%s: no exception #%d (the record holds %d)", recordPath, index+1, len(rec.Exceptions))
		}
	}
	text := RemoveExceptions(string(data), indexes)
	pruned, err := Parse([]byte(text))
	if err != nil {
		return 0, 0, fmt.Errorf("%s: pruned record does not parse: %w", recordPath, err)
	}
	remaining = len(pruned.Exceptions)
	if rec.Ratchet != nil {
		lowered, ok := SetHighWater(text, remaining)
		if !ok {
			return 0, 0, fmt.Errorf("%s: %w", recordPath, errNoHighWater)
		}
		text = lowered
	}
	if err := writeFile(file, text); err != nil {
		return 0, 0, fmt.Errorf("%s: %w", recordPath, err)
	}
	return len(rec.Exceptions) - remaining, remaining, nil
}

func writeFile(file, text string) error {
	info, err := os.Stat(file)
	if err != nil {
		return fmt.Errorf("stat: %w", err)
	}
	dir, err := os.OpenRoot(filepath.Dir(file))
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	writeErr := dir.WriteFile(filepath.Base(file), []byte(text), info.Mode().Perm())
	if closeErr := dir.Close(); writeErr == nil && closeErr != nil {
		return fmt.Errorf("close: %w", closeErr)
	}
	if writeErr != nil {
		return fmt.Errorf("write: %w", writeErr)
	}
	return nil
}

// previousRecord is the record as committed before this change: the one with the lower ceiling of
// HEAD and the merge base with base. A revision that resolves but does not hold the record yields
// an empty record, so creating a record counts as a raise from zero. It is nil when git, the
// repository or any commit is unavailable; baseUnresolved reports that the merge base could not be
// found, so only HEAD was compared.
func previousRecord(root, recordPath, base string) (best *Record, baseUnresolved bool) {
	revisions := []string{"HEAD"}
	if mergeBase, ok := git(root, "merge-base", "--end-of-options", base, "HEAD"); ok {
		revisions = append(revisions, strings.TrimSpace(mergeBase))
	} else {
		baseUnresolved = true
	}
	for _, rev := range revisions {
		rec, ok := recordAt(root, rev, recordPath)
		if !ok {
			continue
		}
		if best == nil || Ceiling(rec) < Ceiling(*best) {
			best = &rec
		}
	}
	return best, baseUnresolved
}

// headResolves reports whether HEAD names a commit.
func headResolves(root string) bool {
	_, ok := git(root, "rev-parse", "--verify", "--quiet", "HEAD^{commit}")
	return ok
}

// recordAt reads the record at revision rev. Paths are resolved from root, not from the top of the
// repository, so a module in a subdirectory finds its own record. ok is false when the revision
// does not resolve or the record at it does not parse; a record absent at a resolving revision is
// the empty record.
func recordAt(root, rev, recordPath string) (Record, bool) {
	if _, ok := git(root, "rev-parse", "--verify", "--quiet", rev+"^{commit}"); !ok {
		return Record{}, false
	}
	listed, ok := git(root, "ls-tree", "--name-only", rev, "--", recordPath)
	if !ok {
		return Record{}, false
	}
	if strings.TrimSpace(listed) == "" {
		return Record{}, true
	}
	text, ok := git(root, "show", rev+":./"+recordPath)
	if !ok {
		return Record{}, false
	}
	rec, err := Parse([]byte(text))
	if err != nil {
		return Record{}, false
	}
	return rec, true
}

// git runs git in dir and returns its standard output; ok is false when git cannot be run or exits
// non-zero, which for history lookups means the history is not there.
func git(dir string, args ...string) (string, bool) {
	cmd := exec.CommandContext(context.Background(), "git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", false
	}
	return string(out), true
}
