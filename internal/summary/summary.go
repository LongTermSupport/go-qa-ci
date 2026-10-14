// Package summary writes the active defences into a delimited region of a file agents already
// load (TOOLING-SPEC 5.3), and judges whether that region is current. Text outside the markers is
// never touched.
package summary

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/LongTermSupport/go-qa-ci/internal/defences"
	"github.com/LongTermSupport/go-qa-ci/internal/finding"
	"github.com/LongTermSupport/go-qa-ci/internal/record"
)

// Markers delimiting the generated region.
const (
	Begin = "<!-- BEGIN go-qa-ci generated defences: run `go-qa summary` to refresh; do not edit -->"
	End   = "<!-- END go-qa-ci generated defences -->"
)

const (
	fileMode = 0o600
	dirMode  = 0o750
)

// Render is the region for a listing, markers included, ending with a newline.
func Render(l defences.Listing) string {
	var out strings.Builder
	out.WriteString(Begin)
	out.WriteString("\n\nActive defences. Failures print these ids unaltered; resolve any id offline with ")
	out.WriteString("`go-qa rule-doc <id>`. Exceptions only via the project record, with a justification.\n\n")
	for _, d := range l.Defences {
		said := ""
		if d.Description != "" {
			said = " " + strings.TrimRight(d.Description, ".") + "."
		}
		if d.Fix != "" {
			said += " Fix: " + d.Fix
		}
		fmt.Fprintf(&out, "- `%s` (%s, %s):%s `go-qa rule-doc %s`\n", d.ID, d.Engine, d.Level, said, d.ID)
	}
	fmt.Fprintf(&out, "\nGate steps, in order (`go-qa tools`): %s.\n", strings.Join(enabledNames(l), ", "))
	out.WriteString("\nProject-record exceptions (prefer fixing the code; an exception names its hazard and scope):\n\n")
	if len(l.Record) == 0 {
		out.WriteString("- none\n")
	}
	for _, e := range l.Record {
		review := "no review date"
		if e.ReviewBy != "" {
			review = "review by " + e.ReviewBy
		}
		model := e.AddedByModel
		if model == "" {
			model = "unknown"
		}
		covers := ""
		if e.Covers == record.CoversFindings {
			covers = ", covers findings"
		}
		fmt.Fprintf(&out, "- `%s` at `%s` (%s, added by %s%s)\n", e.Rule, e.Path, review, model, covers)
	}
	out.WriteString("\nRecord budget (`go-qa record list`):\n\n")
	for _, line := range l.Budget.Lines() {
		fmt.Fprintf(&out, "- %s\n", line)
	}
	out.WriteString("\n" + End + "\n")
	return out.String()
}

func enabledNames(l defences.Listing) []string {
	var names []string
	for _, s := range append(append([]defences.Step{}, l.Stages...), l.Runners...) {
		if s.Enabled {
			names = append(names, s.Name)
		}
	}
	return names
}

// Splice replaces the region in existing with region, or appends it after a blank line when there
// is none. Markers that do not form exactly one region are an error: guessing which text to
// overwrite could destroy what a person wrote.
func Splice(existing, region string) (string, error) {
	begins, ends := strings.Count(existing, Begin), strings.Count(existing, End)
	switch {
	case begins == 0 && ends == 0:
		out := strings.TrimRight(existing, "\n")
		if out != "" {
			out += "\n\n"
		}
		return out + region, nil
	case begins != 1 || ends != 1:
		return "", fmt.Errorf("the file has %d begin and %d end markers; it must have exactly one pair or none", begins, ends)
	}
	start, stop := strings.Index(existing, Begin), strings.Index(existing, End)
	if start > stop {
		return "", errors.New("the end marker comes before the begin marker")
	}
	after := strings.TrimPrefix(existing[stop+len(End):], "\n")
	return existing[:start] + region + after, nil
}

// read returns the file's text; exists is false when it is absent.
func read(root, file string) (text string, exists bool, err error) {
	handle, err := os.OpenRoot(root)
	if err != nil {
		return "", false, fmt.Errorf("open project root: %w", err)
	}
	data, readErr := handle.ReadFile(filepath.FromSlash(file))
	if closeErr := handle.Close(); closeErr != nil {
		return "", false, fmt.Errorf("close project root: %w", closeErr)
	}
	switch {
	case errors.Is(readErr, fs.ErrNotExist):
		return "", false, nil
	case readErr != nil:
		return "", false, fmt.Errorf("read %s: %w", file, readErr)
	}
	return string(data), true, nil
}

// Check judges the region in file (relative to root) against the listing. stale is false when the
// region is current. A missing file, a missing region and an out-of-date region are all
// gqaci.summary.stale, reported in f.
func Check(root, file string, l defences.Listing) (f finding.Finding, stale bool, err error) {
	existing, exists, err := read(root, file)
	if err != nil {
		return finding.Finding{}, false, err
	}
	if !exists {
		return staleFinding(file, "the file does not exist"), true, nil
	}
	want, err := Splice(existing, Render(l))
	if err != nil {
		return finding.Finding{}, false, fmt.Errorf("%s: %w", file, err)
	}
	if want != existing {
		return staleFinding(file, "the generated region is missing or out of date"), true, nil
	}
	return finding.Finding{}, false, nil
}

func staleFinding(file, why string) finding.Finding {
	return finding.Finding{
		Rule: finding.SummaryStale, Path: file,
		Message: why + "; run `go-qa summary` and commit the regenerated region",
	}
}

// Write puts the region for the listing into file (relative to root), creating the file when it
// does not exist and keeping everything outside the markers.
func Write(root, file string, l defences.Listing) error {
	existing, _, err := read(root, file)
	if err != nil {
		return err
	}
	text, err := Splice(existing, Render(l))
	if err != nil {
		return fmt.Errorf("%s: %w", file, err)
	}
	handle, err := os.OpenRoot(root)
	if err != nil {
		return fmt.Errorf("open project root: %w", err)
	}
	writeErr := write(handle, file, text)
	if closeErr := handle.Close(); writeErr == nil && closeErr != nil {
		return fmt.Errorf("close project root: %w", closeErr)
	}
	return writeErr
}

func write(handle *os.Root, file, text string) error {
	if dir := path.Dir(file); dir != "." {
		if err := handle.MkdirAll(filepath.FromSlash(dir), dirMode); err != nil {
			return fmt.Errorf("create %s: %w", dir, err)
		}
	}
	if err := handle.WriteFile(filepath.FromSlash(file), []byte(text), fileMode); err != nil {
		return fmt.Errorf("write %s: %w", file, err)
	}
	return nil
}
