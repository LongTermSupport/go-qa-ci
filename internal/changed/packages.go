package changed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
}

// packageSet is every package of the workspace's modules, keyed by import path and by its
// root-relative directory.
type packageSet struct {
	byImport map[string]*goPackage
	byDir    map[string]*goPackage
}

type listed struct {
	ImportPath   string
	Dir          string
	Imports      []string
	TestImports  []string
	XTestImports []string
}

// listPackages runs go list in every module.
func listPackages(ctx context.Context, root string, modules []string) (packageSet, error) {
	set := packageSet{byImport: map[string]*goPackage{}, byDir: map[string]*goPackage{}}
	for _, module := range modules {
		dir := filepath.Join(root, filepath.FromSlash(module))
		var stdout, stderr bytes.Buffer
		cmd := exec.CommandContext(ctx, "go", "list", "-json=ImportPath,Dir,Imports,TestImports,XTestImports", "./...")
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
			}
			if rel == "." {
				pkg.pattern = "."
			}
			set.byImport[entry.ImportPath] = pkg
			set.byDir[path.Join(module, rel)] = pkg
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
// of every package that imports one of them, directly or not. A changed file whose directory is
// no package is returned as orphan: it was deleted with its package, or is excluded by build
// constraints, and either way the importers cannot be known.
func (s packageSet) affectedBy(goFiles []string) (affected map[string][]string, orphan string) {
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
			return nil, file
		}
		queue = append(queue, pkg.importPath)
	}
	seen := map[string]bool{}
	for len(queue) > 0 {
		importPath := queue[0]
		queue = queue[1:]
		if seen[importPath] {
			continue
		}
		seen[importPath] = true
		queue = append(queue, importers[importPath]...)
	}
	affected = map[string][]string{}
	for importPath := range seen {
		pkg := s.byImport[importPath]
		affected[pkg.module] = append(affected[pkg.module], pkg.pattern)
	}
	for module := range affected {
		slices.Sort(affected[module])
	}
	return affected, ""
}
