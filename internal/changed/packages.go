package changed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/LongTermSupport/go-qa-ci/internal/relpath"
)

// goPackage is a package of the workspace.
type goPackage struct {
	importPath string
	module     string
	pattern    string
	imports    []string
	// testOnly is a package with no production file: it has tests and no statements to count.
	testOnly bool
}

// packageSet is every package of the workspace's modules, keyed by import path and by its
// root-relative directory.
type packageSet struct {
	byImport map[string]*goPackage
	byDir    map[string]*goPackage
	byEmbed  map[string]*goPackage
}

type listed struct {
	ImportPath      string
	Dir             string
	GoFiles         []string
	CgoFiles        []string
	Imports         []string
	TestImports     []string
	XTestImports    []string
	EmbedFiles      []string
	TestEmbedFiles  []string
	XTestEmbedFiles []string
}

// listPackages runs go list in every module.
func listPackages(ctx context.Context, root string, modules []string) (packageSet, error) {
	set := packageSet{byImport: map[string]*goPackage{}, byDir: map[string]*goPackage{}, byEmbed: map[string]*goPackage{}}
	for _, module := range modules {
		dir := filepath.Join(root, filepath.FromSlash(module))
		var stdout, stderr bytes.Buffer
		cmd := exec.CommandContext(ctx, "go", "list", "-json=ImportPath,Dir,GoFiles,CgoFiles,Imports,TestImports,XTestImports,EmbedFiles,TestEmbedFiles,XTestEmbedFiles", "./...")
		cmd.Dir = dir
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			return packageSet{}, fmt.Errorf("go list in %s: %w: %s", displayModule(module), err, strings.TrimSpace(stderr.String()))
		}
		decoder := json.NewDecoder(&stdout)
		for {
			var entry listed
			err := decoder.Decode(&entry)
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return packageSet{}, fmt.Errorf("reading go list output of %s: %w", displayModule(module), err)
			}
			rel, ok := relpath.Within(dir, entry.Dir)
			if !ok {
				return packageSet{}, fmt.Errorf("go list reported %s outside the module %s", entry.Dir, displayModule(module))
			}
			pkg := &goPackage{
				importPath: entry.ImportPath,
				module:     module,
				pattern:    "./" + rel,
				imports:    slices.Concat(entry.Imports, entry.TestImports, entry.XTestImports),
				testOnly:   len(entry.GoFiles) == 0 && len(entry.CgoFiles) == 0,
			}
			if rel == "." {
				pkg.pattern = "."
			}
			set.byImport[entry.ImportPath] = pkg
			set.byDir[path.Join(module, rel)] = pkg
			for _, file := range slices.Concat(entry.EmbedFiles, entry.TestEmbedFiles, entry.XTestEmbedFiles) {
				set.byEmbed[path.Join(module, rel, filepath.ToSlash(file))] = pkg
			}
		}
	}
	return set, nil
}

func displayModule(module string) string {
	if module == "" {
		return "."
	}
	return module
}

// affectedBy returns, per module, the patterns of the packages containing the changed Go files and
// of every package that imports one of them, directly or not. changed holds the first kind alone,
// less test-only packages, which have no statements of their own. A changed file whose directory
// is no package is returned as orphan: it was deleted with its package, or is excluded by build
// constraints, and either way the importers cannot be known.
//
// Files that are no Go source of the package still decide its result: an embedded file changes the
// package and so its importers; a file under testdata changes only the tests of the package that
// owns the directory. Both are mapped to their package; a file that belongs to none is ignored.
func (s packageSet) affectedBy(goFiles, otherFiles []string) (affected, changed map[string][]string, orphan string) {
	importers := map[string][]string{}
	for importPath, pkg := range s.byImport {
		for _, imported := range pkg.imports {
			if _, inWorkspace := s.byImport[imported]; inWorkspace {
				importers[imported] = append(importers[imported], importPath)
			}
		}
	}
	var queue []string
	for _, file := range goFiles {
		pkg, ok := s.byDir[path.Dir(file)]
		if !ok {
			return nil, nil, file
		}
		queue = append(queue, pkg.importPath)
	}
	var direct []string
	for _, file := range otherFiles {
		if pkg, ok := s.byEmbed[file]; ok {
			queue = append(queue, pkg.importPath)
		} else if pkg := s.testdataOwner(file); pkg != nil {
			direct = append(direct, pkg.importPath)
		}
	}
	changed = s.byModule(slices.DeleteFunc(slices.Concat(direct, queue), func(importPath string) bool {
		return s.byImport[importPath].testOnly
	}))
	seen := map[string]bool{}
	for _, importPath := range direct {
		seen[importPath] = true
	}
	for len(queue) > 0 {
		importPath := queue[0]
		queue = queue[1:]
		if seen[importPath] {
			continue
		}
		seen[importPath] = true
		queue = append(queue, importers[importPath]...)
	}
	return s.byModule(slices.Collect(maps.Keys(seen))), changed, ""
}

// byModule groups the patterns of the packages with the given import paths by module, sorted and
// without repeats.
func (s packageSet) byModule(importPaths []string) map[string][]string {
	patterns := map[string][]string{}
	for _, importPath := range importPaths {
		pkg := s.byImport[importPath]
		patterns[pkg.module] = append(patterns[pkg.module], pkg.pattern)
	}
	for module := range patterns {
		slices.Sort(patterns[module])
		patterns[module] = slices.Compact(patterns[module])
	}
	return patterns
}

// testdataOwner is the package whose directory holds the testdata directory that contains file.
func (s packageSet) testdataOwner(file string) *goPackage {
	dir := path.Dir(file)
	for {
		parent, last := path.Split(dir)
		parent = path.Clean(parent)
		if last == "testdata" {
			for owner := parent; ; owner = path.Dir(owner) {
				if pkg, ok := s.byDir[owner]; ok {
					return pkg
				}
				if owner == "." || owner == "/" {
					return nil
				}
			}
		}
		if parent == dir || dir == "." {
			return nil
		}
		dir = parent
	}
}
