// Package declaration reads the machine-readable Defence Before Fix declaration
// (defence-before-fix.toml) and reconciles it with the clause-by-clause page
// (docs/CONFORMANCE.md), TOOLING-SPEC 9.1 and 9.2.
//
// The declaration names the spec versions followed and, for each of the two levels of 9.1 (the
// project and the shipped artefact), whether it claims conformance and every known gap. A level
// with a gap claims nothing; a level without one claims conformance, which only the evidence on
// the page can support.
package declaration

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
)

// Specs are the versions of the three specifications the declaration is made against.
type Specs struct {
	Method    string `toml:"method"`
	Toolchain string `toml:"toolchain"`
	Detector  string `toml:"detector"`
}

// Gap is one known failure of a MUST (or SHOULD) of the toolchain specification, recorded
// alongside the version (TOOLING-SPEC 9.2).
type Gap struct {
	Clause  string `toml:"clause"`
	Summary string `toml:"summary"`
	// Record is the file that holds the full account.
	Record string `toml:"record"`
}

// Level is the declaration for one of the two levels of 9.1.
type Level struct {
	Conforms bool  `toml:"conforms"`
	Gaps     []Gap `toml:"gap"`
}

// Declaration is the content of defence-before-fix.toml.
type Declaration struct {
	Specs    Specs `toml:"specs"`
	Project  Level `toml:"project"`
	Artefact Level `toml:"artefact"`
}

var (
	versionPattern = regexp.MustCompile(`^\d+\.\d+\.\d+$`)
	clausePattern  = regexp.MustCompile(`^\d+\.\d+$`)
)

// Parse reads and validates a declaration; unknown keys are rejected.
func Parse(data []byte) (Declaration, error) {
	var d Declaration
	dec := toml.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&d); err != nil {
		var strict *toml.StrictMissingError
		if errors.As(err, &strict) {
			return Declaration{}, fmt.Errorf("declaration: %s", strict.String())
		}
		return Declaration{}, fmt.Errorf("declaration: %w", err)
	}
	if err := d.validate(); err != nil {
		return Declaration{}, fmt.Errorf("declaration: %w", err)
	}
	return d, nil
}

func (d Declaration) validate() error {
	for name, version := range map[string]string{
		"specs.method": d.Specs.Method, "specs.toolchain": d.Specs.Toolchain, "specs.detector": d.Specs.Detector,
	} {
		if !versionPattern.MatchString(version) {
			return fmt.Errorf("%s = %q is not a version x.y.z", name, version)
		}
	}
	if err := d.Project.validate("project", false); err != nil {
		return err
	}
	return d.Artefact.validate("artefact", true)
}

func (l Level) validate(name string, artefact bool) error {
	seen := map[string]bool{}
	for _, g := range l.Gaps {
		if !clausePattern.MatchString(g.Clause) {
			return fmt.Errorf("%s: gap clause %q is not a clause number such as 4.1", name, g.Clause)
		}
		if seen[g.Clause] {
			return fmt.Errorf("%s: clause %s is recorded twice", name, g.Clause)
		}
		seen[g.Clause] = true
		if strings.HasPrefix(g.Clause, "8.") && !artefact {
			return fmt.Errorf("%s: clause %s is in section 8, which bears on the artefact grade only (TOOLING-SPEC 9.1)", name, g.Clause)
		}
		if strings.TrimSpace(g.Summary) == "" {
			return fmt.Errorf("%s: gap %s has no summary", name, g.Clause)
		}
		if strings.TrimSpace(g.Record) == "" {
			return fmt.Errorf("%s: gap %s names no record file", name, g.Clause)
		}
	}
	switch {
	case l.Conforms && len(l.Gaps) > 0:
		return fmt.Errorf("%s: conforms = true with %d recorded gaps; a non-empty gap record is not a claim (TOOLING-SPEC 9.2)", name, len(l.Gaps))
	case !l.Conforms && len(l.Gaps) == 0:
		return fmt.Errorf("%s: conforms = false needs a gap that says where it fails (TOOLING-SPEC 9.2)", name)
	}
	return nil
}
