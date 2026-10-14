package changed

import (
	"path"
	"slices"
	"strings"
)

// wideningNames are the files whose change can alter the result for every package: module and
// workspace definitions, the tool lock and the golangci configuration.
var wideningNames = []string{
	"go.mod", "go.sum", "go.work", "go.work.sum", "tools.lock",
	".golangci.yml", ".golangci.yaml", ".golangci.toml", ".golangci.json",
}

// widens reports whether a change to file calls for a run over the whole project: it is a
// widening name, one of the extra files, or inside one of the extra directories. Which files and
// directories those are is the project's configuration, never assumed here.
func widens(file string, extraFiles, extraDirs []string) bool {
	return slices.Contains(wideningNames, path.Base(file)) ||
		slices.Contains(extraFiles, file) ||
		slices.ContainsFunc(extraDirs, func(dir string) bool { return within(file, dir) })
}

// within reports whether file lies inside dir; "." and "" are the whole project.
func within(file, dir string) bool {
	dir = path.Clean(dir)
	return dir == "." || strings.HasPrefix(file, dir+"/")
}

// ignoredByGo reports whether the go command never builds file as part of a package: it lies under
// testdata, vendor, or a directory whose name starts with "_" or ".".
func ignoredByGo(file string) bool {
	for part := range strings.SplitSeq(path.Dir(file), "/") {
		if part == "testdata" || part == "vendor" || strings.HasPrefix(part, "_") || (strings.HasPrefix(part, ".") && part != ".") {
			return true
		}
	}
	return false
}
