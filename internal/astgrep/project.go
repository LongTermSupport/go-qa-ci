// Package astgrep reads an ast-grep project (sgconfig.yml, its rules and their test cases), the
// bundled rule pack and the scan output, so the ast-grep detector can judge them without
// shelling out for anything but the scan itself.
package astgrep

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"slices"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

// ConfigFile is the file name that marks a project as an ast-grep project.
const ConfigFile = "sgconfig.yml"

// Project is the part of sgconfig.yml the detector needs.
type Project struct {
	// RuleDirs are the rule directories, slash-separated and relative to the project root.
	RuleDirs []string
	// TestDirs are the test-case directories of testConfigs, relative to the project root.
	TestDirs []string
}

type rawConfig struct {
	RuleDirs    []string `yaml:"ruleDirs"`
	TestConfigs []struct {
		TestDir string `yaml:"testDir"`
	} `yaml:"testConfigs"`
}

// LoadProject reads root/sgconfig.yml. found is false when the file does not exist. Every
// directory it names must lie inside root.
func LoadProject(root string) (project Project, found bool, err error) {
	handle, err := os.OpenRoot(root)
	if err != nil {
		return Project{}, false, fmt.Errorf("opening the project root: %w", err)
	}
	defer func() { err = errors.Join(err, handle.Close()) }()
	text, err := handle.ReadFile(ConfigFile)
	if errors.Is(err, fs.ErrNotExist) {
		return Project{}, false, nil
	}
	if err != nil {
		return Project{}, false, fmt.Errorf("reading %s: %w", ConfigFile, err)
	}
	var raw rawConfig
	if parseErr := yaml.Unmarshal(text, &raw); parseErr != nil {
		return Project{}, true, fmt.Errorf("%s: %w", ConfigFile, parseErr)
	}
	project.RuleDirs = slices.Clone(raw.RuleDirs)
	for _, config := range raw.TestConfigs {
		project.TestDirs = append(project.TestDirs, config.TestDir)
	}
	for _, dir := range slices.Concat(project.RuleDirs, project.TestDirs) {
		if !fs.ValidPath(path.Clean(dir)) {
			return Project{}, true, fmt.Errorf("%s: directory %q is outside the project root", ConfigFile, dir)
		}
	}
	return project, true, nil
}

// HasTests reports whether sgconfig.yml declares any test directory.
func (p Project) HasTests() bool {
	return len(p.TestDirs) > 0
}

// Rule is one project rule.
type Rule struct {
	// ID is the rule's identifier exactly as ast-grep prints it.
	ID string
	// Severity is the declared severity; ast-grep's default, hint, when none is given.
	Severity string
	// File is the rule file, slash-separated and relative to the project root.
	File string
}

// Inventory is every rule of a project.
type Inventory struct {
	// Rules are the rules that have an id, in file order.
	Rules []Rule
	// Unnamed are the rule files holding a rule with no id: ast-grep runs it under its file name,
	// which nothing lists, documents or tests.
	Unnamed []string
}

// Inventory reads every rule file in the rule directories.
func (p Project) Inventory(root string) (inventory Inventory, err error) {
	handle, err := os.OpenRoot(root)
	if err != nil {
		return Inventory{}, fmt.Errorf("opening the project root: %w", err)
	}
	defer func() { err = errors.Join(err, handle.Close()) }()
	for _, dir := range p.RuleDirs {
		files, listErr := yamlFiles(handle.FS(), dir)
		if listErr != nil {
			return Inventory{}, fmt.Errorf("rule directory %s: %w", dir, listErr)
		}
		for _, file := range files {
			documents, readErr := readDocuments(handle.FS(), file)
			if readErr != nil {
				return Inventory{}, readErr
			}
			unnamed := false
			for _, document := range documents {
				if document == nil {
					continue
				}
				id := stringField(document, "id")
				if id == "" {
					unnamed = true
					continue
				}
				severity := stringField(document, "severity")
				if severity == "" {
					severity = "hint"
				}
				inventory.Rules = append(inventory.Rules, Rule{ID: id, Severity: severity, File: file})
			}
			if unnamed {
				inventory.Unnamed = append(inventory.Unnamed, file)
			}
		}
	}
	sort.Strings(inventory.Unnamed)
	return inventory, nil
}

// Tested lists, sorted, the identifiers of the rules that have a test case holding both valid
// and invalid code.
func (p Project) Tested(root string) (tested []string, err error) {
	handle, err := os.OpenRoot(root)
	if err != nil {
		return nil, fmt.Errorf("opening the project root: %w", err)
	}
	defer func() { err = errors.Join(err, handle.Close()) }()
	for _, dir := range p.TestDirs {
		files, listErr := yamlFiles(handle.FS(), dir)
		if listErr != nil {
			return nil, fmt.Errorf("test directory %s: %w", dir, listErr)
		}
		for _, file := range files {
			documents, readErr := readDocuments(handle.FS(), file)
			if readErr != nil {
				return nil, readErr
			}
			for _, document := range documents {
				id := stringField(document, "id")
				if id != "" && nonEmptyList(document["valid"]) && nonEmptyList(document["invalid"]) {
					tested = append(tested, id)
				}
			}
		}
	}
	sort.Strings(tested)
	return slices.Compact(tested), nil
}

func nonEmptyList(value any) bool {
	list, isList := value.([]any)
	return isList && len(list) > 0
}

// stringField is the string value of key, or "" when it is absent or not a string.
func stringField(document map[string]any, key string) string {
	value, isString := document[key].(string)
	if !isString {
		return ""
	}
	return value
}

// yamlFiles lists the .yml and .yaml files below dir in fsys, sorted by path.
func yamlFiles(fsys fs.FS, dir string) ([]string, error) {
	var files []string
	walkErr := fs.WalkDir(fsys, path.Clean(dir), func(current string, entry fs.DirEntry, visitErr error) error {
		if visitErr != nil {
			return visitErr
		}
		if !entry.IsDir() && (strings.HasSuffix(current, ".yml") || strings.HasSuffix(current, ".yaml")) {
			files = append(files, current)
		}
		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("walking %s: %w", dir, walkErr)
	}
	sort.Strings(files)
	return files, nil
}

// readDocuments decodes every YAML document of a file; an empty document is nil.
func readDocuments(fsys fs.FS, file string) ([]map[string]any, error) {
	text, err := fs.ReadFile(fsys, file)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", file, err)
	}
	decoder := yaml.NewDecoder(bytes.NewReader(text))
	var documents []map[string]any
	for {
		var document map[string]any
		decodeErr := decoder.Decode(&document)
		if errors.Is(decodeErr, io.EOF) {
			return documents, nil
		}
		if decodeErr != nil {
			return nil, fmt.Errorf("%s: %w", file, decodeErr)
		}
		documents = append(documents, document)
	}
}
