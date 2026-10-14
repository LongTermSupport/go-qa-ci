package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
)

const maxPercent = 100

// Load reads the configuration for root. Without explicit, qa/go-qa.toml is read when present and
// the defaults apply otherwise; an explicit file must exist. Every problem found is reported in
// one error.
func Load(root, explicit string) (Config, error) {
	shown := DefaultPath
	if explicit != "" {
		shown = explicit
	}
	data, err := readInRoot(root, shown)
	switch {
	case errors.Is(err, os.ErrNotExist) && explicit == "":
		return Default(), nil
	case err != nil:
		return Config{}, fmt.Errorf("%s: %w", shown, err)
	}
	cfg, err := decode(data)
	if err != nil {
		return Config{}, fmt.Errorf("%s: %w", shown, err)
	}
	if problems := cfg.problems(root); len(problems) > 0 {
		return Config{}, fmt.Errorf("%s: %s", shown, strings.Join(problems, "; "))
	}
	return cfg, nil
}

// readInRoot reads a file that must lie inside root.
func readInRoot(root, rel string) ([]byte, error) {
	dir, err := os.OpenRoot(root)
	if err != nil {
		return nil, fmt.Errorf("open root: %w", err)
	}
	data, readErr := dir.ReadFile(rel)
	if closeErr := dir.Close(); readErr == nil && closeErr != nil {
		return nil, fmt.Errorf("close root: %w", closeErr)
	}
	if readErr != nil {
		return nil, fmt.Errorf("read: %w", readErr)
	}
	return data, nil
}

// decode applies data over the defaults, rejecting unknown keys.
func decode(data []byte) (Config, error) {
	data, err := expandRecordShorthand(data)
	if err != nil {
		return Config{}, err
	}
	cfg := Default()
	dec := toml.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		var strict *toml.StrictMissingError
		if errors.As(err, &strict) {
			keys := make([]string, 0, len(strict.Errors))
			for _, e := range strict.Errors {
				keys = append(keys, fmt.Sprintf("%q", strings.Join(e.Key(), ".")))
			}
			return Config{}, fmt.Errorf("unknown key %s", strings.Join(keys, ", "))
		}
		return Config{}, fmt.Errorf("invalid configuration: %w", err)
	}
	return cfg, nil
}

// expandRecordShorthand rewrites `record = "<path>"` into `[record] path = "<path>"`, so that the
// table is decoded (and its unknown keys rejected) by the one decoder. Any other document is
// returned unchanged.
func expandRecordShorthand(data []byte) ([]byte, error) {
	var raw map[string]any
	if err := toml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("invalid TOML: %w", err)
	}
	switch record := raw["record"].(type) {
	case nil, map[string]any:
		return data, nil
	case string:
		raw["record"] = map[string]any{"path": record}
		expanded, err := toml.Marshal(raw)
		if err != nil {
			return nil, fmt.Errorf("record: %w", err)
		}
		return expanded, nil
	default:
		return nil, fmt.Errorf("record must be a path or a table, found %T", record)
	}
}

// problems checks the values the decoder cannot: ranges, coherent caps, and paths that stay inside
// the root.
func (c Config) problems(root string) []string {
	var problems []string
	if c.Coverage.FailUnderStatements < 0 || c.Coverage.FailUnderStatements > maxPercent {
		problems = append(problems, fmt.Sprintf("coverage.fail_under_statements must be between 0 and %d", maxPercent))
	}
	if !c.Mutants.Mode.Valid() {
		problems = append(problems, fmt.Sprintf("mutants.mode %q is unknown (want off, diff or full)", c.Mutants.Mode))
	}
	problems = append(problems, c.Record.Problems()...)
	local := map[string]string{"record.path": c.Record.Path, "docs_dir": c.DocsDir}
	if c.Summary.File != "" {
		local["summary.file"] = c.Summary.File
	}
	for key, value := range local {
		if !filepath.IsLocal(value) {
			problems = append(problems, fmt.Sprintf("%s %q must be a relative path inside the root", key, value))
		}
	}
	if driver := c.Analyzers.Driver; driver != "" && (!strings.HasPrefix(driver, "./") || !filepath.IsLocal(driver)) {
		problems = append(problems, fmt.Sprintf("analyzers.driver %q must be a package inside the root, written like ./internal/qa/vetdriver", driver))
	}
	for _, module := range c.Modules {
		switch {
		case !filepath.IsLocal(module):
			problems = append(problems, fmt.Sprintf("modules entry %q must be a relative path inside the root", module))
		case !isFile(filepath.Join(root, module, "go.mod")):
			problems = append(problems, fmt.Sprintf("modules entry %q has no go.mod", module))
		}
	}
	return problems
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}
