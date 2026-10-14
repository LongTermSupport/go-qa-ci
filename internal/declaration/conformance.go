package declaration

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// Row is one clause of docs/CONFORMANCE.md.
type Row struct {
	Clause string
	// Status is the status cell as written, for example "Met", "Met as test", "Partial" or "Gap".
	Status string
}

// Conformance is the parsed clause-by-clause page.
type Conformance struct {
	Specs    Specs
	Project  []Row
	Artefact []Row
}

const (
	projectHeading  = "## Project level"
	artefactHeading = "## Artefact level"
)

var specsLine = regexp.MustCompile(`(?m)^Specs: method (\d+\.\d+\.\d+), detector (\d+\.\d+\.\d+), toolchain (\d+\.\d+\.\d+)`)

// statusWords are the leading words a status cell may start with.
var statusWords = []string{"Met", "Partial", "Gap", "N/A"}

// requiredClauses are the clauses a level must grade: the MUSTs of sections 4 to 6 and the SHOULDs
// of 6.4 and section 7, plus section 8 for the artefact.
var requiredClauses = []string{"4.1", "4.2", "4.3", "4.4", "4.5", "5.1", "5.2", "5.3", "6.1", "6.2", "6.3", "6.4", "7.1", "7.2"}

var artefactClauses = []string{"8.1", "8.2"}

// ParseConformance reads the page's Specs line and the two level tables.
func ParseConformance(markdown string) (Conformance, error) {
	m := specsLine.FindStringSubmatch(markdown)
	if m == nil {
		return Conformance{}, fmt.Errorf("conformance page: no %q line", "Specs: method X, detector Y, toolchain Z.")
	}
	c := Conformance{Specs: Specs{Method: m[1], Detector: m[2], Toolchain: m[3]}}
	var err error
	if c.Project, err = levelRows(markdown, projectHeading); err != nil {
		return Conformance{}, err
	}
	if c.Artefact, err = levelRows(markdown, artefactHeading); err != nil {
		return Conformance{}, err
	}
	return c, nil
}

// levelRows returns the table rows between a heading and the next heading of the same level.
func levelRows(markdown, heading string) ([]Row, error) {
	_, after, ok := strings.Cut(markdown, heading+"\n")
	if !ok {
		return nil, fmt.Errorf("conformance page: no %q section", strings.TrimPrefix(heading, "## "))
	}
	section, _, _ := strings.Cut(after, "\n## ")
	var rows []Row
	seen := map[string]bool{}
	for line := range strings.SplitSeq(section, "\n") {
		if !strings.HasPrefix(line, "|") {
			continue
		}
		cells := strings.Split(strings.Trim(strings.TrimSpace(line), "|"), "|")
		if len(cells) < 3 {
			continue
		}
		clause, status := strings.TrimSpace(cells[0]), strings.TrimSpace(cells[2])
		if !clausePattern.MatchString(clause) {
			continue // the header and separator rows
		}
		if seen[clause] {
			return nil, fmt.Errorf("conformance page: clause %s appears twice in %q", clause, strings.TrimPrefix(heading, "## "))
		}
		seen[clause] = true
		if !slices.ContainsFunc(statusWords, func(w string) bool { return strings.HasPrefix(status, w) }) {
			return nil, fmt.Errorf("conformance page: clause %s has status %q; want one starting with %s", clause, status, strings.Join(statusWords, ", "))
		}
		rows = append(rows, Row{Clause: clause, Status: status})
	}
	return rows, nil
}

func isOpen(status string) bool {
	return strings.HasPrefix(status, "Gap") || strings.HasPrefix(status, "Partial")
}

// Reconcile compares the declaration with the page and returns every disagreement: the spec
// versions, a clause the page leaves out, a Gap or Partial row with no recorded gap, and a
// recorded gap on a clause the page does not mark Gap or Partial.
func Reconcile(d Declaration, c Conformance) []string {
	var problems []string
	for name, pair := range map[string][2]string{
		"method": {d.Specs.Method, c.Specs.Method}, "toolchain": {d.Specs.Toolchain, c.Specs.Toolchain}, "detector": {d.Specs.Detector, c.Specs.Detector},
	} {
		if pair[0] != pair[1] {
			problems = append(problems, fmt.Sprintf("specs: %s is %s in the declaration but %s on the page", name, pair[0], pair[1]))
		}
	}
	problems = append(problems, reconcileLevel("project", d.Project, c.Project, requiredClauses)...)
	problems = append(problems, reconcileLevel("artefact", d.Artefact, c.Artefact, slices.Concat(requiredClauses, artefactClauses))...)
	slices.Sort(problems)
	return problems
}

func reconcileLevel(name string, level Level, rows []Row, required []string) []string {
	var problems []string
	status := map[string]string{}
	for _, r := range rows {
		status[r.Clause] = r.Status
	}
	for _, clause := range required {
		if _, ok := status[clause]; !ok {
			problems = append(problems, fmt.Sprintf("%s: clause %s is required but the page has no row for it", name, clause))
		}
	}
	declared := map[string]bool{}
	for _, g := range level.Gaps {
		declared[g.Clause] = true
		if s, ok := status[g.Clause]; !ok || !isOpen(s) {
			problems = append(problems, fmt.Sprintf("%s: gap %s is declared but the page marks it %s", name, g.Clause, orMissing(s)))
		}
	}
	for _, r := range rows {
		if isOpen(r.Status) && !declared[r.Clause] {
			problems = append(problems, fmt.Sprintf("%s: clause %s is %s in the page but has no gap in the declaration", name, r.Clause, firstWord(r.Status)))
		}
	}
	return problems
}

func orMissing(s string) string {
	if s == "" {
		return "absent"
	}
	return firstWord(s)
}

func firstWord(s string) string {
	word, _, _ := strings.Cut(s, " ")
	return word
}
