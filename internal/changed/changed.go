// Package changed computes the scope of a diff-mode run (`go-qa run --changed`): which files
// changed, which packages they put at risk (the packages containing them and every package that
// imports those, within the workspace's modules) and whether a change is wide enough that only a
// run over the whole project is honest.
//
// Diff mode is for fast local iteration. It narrows the work of the per-package engines and the
// file-scoped detectors; the full run remains the gate before a push.
package changed

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

// Options says what to compare and which changes widen the run.
type Options struct {
	// Root is the project root.
	Root string
	// Base is the ref to compare against; empty means HEAD. A ref selects the files changed since
	// the merge base of Base and HEAD, working tree included.
	Base string
	// Modules lists the module directories, relative to Root and slash-separated; "" is the root
	// module.
	Modules []string
	// Widen lists further project-relative paths whose change widens the run (the project record
	// and the go-qa configuration, wherever the project put them).
	Widen []string
	// WidenDirs lists project-relative directories whose every file widens the run when changed:
	// the rule sources the project's detectors read (ast-grep rule and test directories, the
	// documentation directory, the analyzer module or driver package).
	WidenDirs []string
}

// Plan is what a diff-mode run covers.
type Plan struct {
	// Base is the ref compared against; empty means HEAD.
	Base string
	// Files are the changed files, relative to the root and slash-separated, deleted ones
	// included.
	Files []string
	// Scope are the changed files that still exist: what a file-scoped detector can read.
	Scope []string
	// Widened is why the run covers the whole project; empty for a narrowed run.
	Widened string
	// Packages maps a module directory to the package patterns ("./a/b", relative to the module)
	// of the changed packages and their importers.
	Packages map[string][]string
	// Changed maps a module directory to the patterns of the changed packages alone, without
	// their importers: the code a narrowed coverage measurement judges.
	Changed map[string][]string
}

// Narrow reports whether the run is limited to the changed files and packages. A nil plan is the
// full run.
func (p *Plan) Narrow() bool { return p != nil && p.Widened == "" }

// GoFiles are the existing changed Go source files.
func (p *Plan) GoFiles() []string {
	return slices.DeleteFunc(slices.Clone(p.Scope), func(file string) bool { return !strings.HasSuffix(file, ".go") })
}

// Describe is the mode and scope lines of the run summary.
func (p *Plan) Describe() string {
	var out strings.Builder
	if p.Base == "" {
		out.WriteString("mode: changed (files changed against HEAD, plus untracked files that are not ignored)\n")
	} else {
		fmt.Fprintf(&out, "mode: changed (files changed since the merge base of %s and HEAD, plus the working tree)\n", p.Base)
	}
	switch {
	case p.Widened != "":
		fmt.Fprintf(&out, "scope: whole project (widened: %s)\n", p.Widened)
	case len(p.Files) == 0:
		out.WriteString("scope: no changed files; only the repo-wide defences run\n")
	default:
		fmt.Fprintf(&out, "scope: %s", plural(len(p.Files), "changed file"))
		if packages := p.packageList(); len(packages) > 0 {
			fmt.Fprintf(&out, "; packages: %s", strings.Join(packages, ", "))
		} else {
			out.WriteString("; no Go packages")
		}
		out.WriteString("\n")
	}
	return out.String()
}

func plural(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// packageList names every affected package, root-relative, in module then pattern order.
func (p *Plan) packageList() []string {
	modules := make([]string, 0, len(p.Packages))
	for module := range p.Packages {
		modules = append(modules, module)
	}
	slices.Sort(modules)
	var names []string
	for _, module := range modules {
		for _, pattern := range p.Packages[module] {
			if module == "" {
				names = append(names, pattern)
				continue
			}
			names = append(names, "./"+path.Join(module, pattern))
		}
	}
	return names
}

// Compute works out the plan for a run.
func Compute(ctx context.Context, opts Options) (*Plan, error) {
	if strings.HasPrefix(opts.Base, "-") {
		return nil, fmt.Errorf("--base %q must be a git ref, not an option", opts.Base)
	}
	files, err := changedFiles(ctx, opts.Root, opts.Base)
	if err != nil {
		return nil, err
	}
	plan := &Plan{Base: opts.Base, Files: files}
	for _, file := range files {
		info, statErr := os.Lstat(filepath.Join(opts.Root, filepath.FromSlash(file)))
		if statErr == nil && info.Mode().IsRegular() {
			plan.Scope = append(plan.Scope, file)
		}
	}
	for _, file := range files {
		if widens(file, opts.Widen, opts.WidenDirs) {
			plan.Widened = file + " changed"
			return plan, nil
		}
	}
	goFiles := slices.DeleteFunc(slices.Clone(files), func(file string) bool {
		return !strings.HasSuffix(file, ".go") || ignoredByGo(file)
	})
	otherFiles := slices.DeleteFunc(slices.Clone(files), func(file string) bool {
		return slices.Contains(goFiles, file) || (strings.HasSuffix(file, ".go") && !strings.Contains("/"+path.Dir(file)+"/", "/testdata/"))
	})
	if len(goFiles) == 0 && len(otherFiles) == 0 {
		return plan, nil
	}
	packages, err := listPackages(ctx, opts.Root, opts.Modules)
	if err != nil {
		return nil, err
	}
	affected, direct, orphan := packages.affectedBy(goFiles, otherFiles)
	if orphan != "" {
		plan.Widened = orphan + " changed outside any package that still exists"
		return plan, nil
	}
	plan.Packages, plan.Changed = affected, direct
	return plan, nil
}
