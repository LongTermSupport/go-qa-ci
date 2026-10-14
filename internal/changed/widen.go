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

// widenDir is the directory holding go-qa's configuration and the project record by default.
const widenDir = "qa/"

// widens reports whether a change to file calls for a run over the whole project.
func widens(file string, extra []string) bool {
	return slices.Contains(wideningNames, path.Base(file)) ||
		strings.HasPrefix(file, widenDir) ||
		slices.Contains(extra, file)
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
