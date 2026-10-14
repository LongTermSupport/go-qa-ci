package astgrep

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

// PackPrefix is the namespace every bundled rule identifier lives in.
const PackPrefix = "gqaci.ast."

// PackFile is the name the pack is written under.
const PackFile = "gqaci-pack.yml"

//go:embed pack
var packFiles embed.FS

// Pack is the bundled rule pack: a few single-file syntactic rules where syntax provably suffices
// and no enabled linter covers the same ground.
type Pack struct {
	rules []packRule
}

type packRule struct {
	id      string
	text    string
	message string
}

// Bundled loads the pack shipped inside the binary.
func Bundled() (Pack, error) {
	files, err := fs.Sub(packFiles, "pack")
	if err != nil {
		return Pack{}, fmt.Errorf("opening the embedded pack: %w", err)
	}
	return LoadPack(files)
}

// LoadPack reads every .yml file at the top of files as one bundled rule and checks the pack's
// invariants: one rule per file, the file named after the rule, the identifier in the gqaci.ast.
// namespace, Go, error severity, and a note that points at the rule's page.
func LoadPack(files fs.FS) (Pack, error) {
	entries, err := fs.ReadDir(files, ".")
	if err != nil {
		return Pack{}, fmt.Errorf("reading the pack: %w", err)
	}
	var pack Pack
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".yml") {
			continue
		}
		text, err := fs.ReadFile(files, name)
		if err != nil {
			return Pack{}, fmt.Errorf("reading %s: %w", name, err)
		}
		rule, err := checkPackRule(name, text)
		if err != nil {
			return Pack{}, err
		}
		pack.rules = append(pack.rules, packRule{id: rule.ID, text: string(text), message: rule.Message})
	}
	sort.Slice(pack.rules, func(i, j int) bool { return pack.rules[i].id < pack.rules[j].id })
	return pack, nil
}

type rawPackRule struct {
	ID       string `yaml:"id"`
	Language string `yaml:"language"`
	Severity string `yaml:"severity"`
	Note     string `yaml:"note"`
	Message  string `yaml:"message"`
}

func checkPackRule(name string, text []byte) (rawPackRule, error) {
	var rule rawPackRule
	if err := yaml.Unmarshal(text, &rule); err != nil {
		return rawPackRule{}, fmt.Errorf("bundled rule %s: %w", name, err)
	}
	switch {
	case !strings.HasPrefix(rule.ID, PackPrefix):
		return rawPackRule{}, fmt.Errorf("bundled rule %s: id %q is not in the %s namespace", name, rule.ID, PackPrefix)
	case name != rule.ID+".yml":
		return rawPackRule{}, fmt.Errorf("bundled rule %s: the file must be named %s.yml", name, rule.ID)
	case rule.Language != "go":
		return rawPackRule{}, fmt.Errorf("bundled rule %s: language is %q, not go", name, rule.Language)
	case rule.Severity != "error":
		return rawPackRule{}, fmt.Errorf("bundled rule %s: severity is %q; a bundled rule blocks", name, rule.Severity)
	case !strings.Contains(rule.Note, "go-qa rule-doc "+rule.ID):
		return rawPackRule{}, fmt.Errorf("bundled rule %s: its note must point at its page with `go-qa rule-doc %s`", name, rule.ID)
	}
	return rule, nil
}

// IDs are the identifiers of the bundled rules, sorted.
func (p Pack) IDs() []string {
	ids := make([]string, len(p.rules))
	for i, rule := range p.rules {
		ids[i] = rule.id
	}
	return ids
}

// Empty reports whether the pack has no rules, so there is nothing to scan with.
func (p Pack) Empty() bool {
	return len(p.rules) == 0
}

// Write writes the pack to dir as one multi-document file, as `ast-grep scan --rule` accepts it,
// and returns its path.
func (p Pack) Write(dir string) (string, error) {
	if p.Empty() {
		return "", errors.New("the bundled pack has no rules to write")
	}
	texts := make([]string, len(p.rules))
	for i, rule := range p.rules {
		texts[i] = strings.TrimSuffix(rule.text, "\n") + "\n"
	}
	target := filepath.Join(dir, PackFile)
	if err := os.WriteFile(target, []byte(strings.Join(texts, "---\n")), 0o600); err != nil {
		return "", fmt.Errorf("writing the pack: %w", err)
	}
	return target, nil
}

// Rules describes the bundled rules, sorted by identifier. Every bundled rule blocks.
func (p Pack) Rules() []Rule {
	rules := make([]Rule, len(p.rules))
	for i, rule := range p.rules {
		rules[i] = Rule{ID: rule.id, Severity: "error", File: rule.id + ".yml", Message: rule.message}
	}
	return rules
}
