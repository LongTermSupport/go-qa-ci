package golangci

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
)

// Linter is one enabled linter, as golangci-lint describes it.
type Linter struct {
	// Name is the identifier its findings are printed under.
	Name string
	// Description says what the linter finds.
	Description string
}

type lintersOutput struct {
	Enabled *[]struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	} `json:"Enabled"`
}

// ParseLinters reads `golangci-lint linters --json` into the enabled linters, sorted by name. The
// output is another program's format, so unknown fields are ignored, but a document without an
// enabled list is an error: an empty listing must never be mistaken for "nothing is enabled".
func ParseLinters(data []byte) ([]Linter, error) {
	var out lintersOutput
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("reading the %s linter listing: %w", binary, err)
	}
	if out.Enabled == nil {
		return nil, errors.New("the " + binary + " linter listing has no Enabled list")
	}
	linters := make([]Linter, len(*out.Enabled))
	for i, l := range *out.Enabled {
		linters[i] = Linter{Name: l.Name, Description: strings.TrimSpace(l.Description)}
	}
	slices.SortFunc(linters, func(a, b Linter) int { return strings.Compare(a.Name, b.Name) })
	return linters, nil
}

// Linters lists the linters the configuration enables for the module in dir.
func (r Runner) Linters(ctx context.Context, dir, configPath string) ([]Linter, error) {
	args := append([]string{"linters", "--json"}, configArgs(configPath)...)
	stdout, stderr, code, err := r.run(ctx, dir, args)
	if err != nil {
		return nil, err
	}
	if code != 0 {
		return nil, fmt.Errorf("%s linters exited %d: %s", binary, code, strings.TrimSpace(stderr))
	}
	return ParseLinters([]byte(stdout))
}
