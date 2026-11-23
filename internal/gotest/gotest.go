// Package gotest knows which go test flags defeat Go's test cache, for the runner that must not
// pass them on a cacheable run and the stage check that refuses them outside a full run.
package gotest

import (
	"slices"
	"strings"
)

// rerunFlagNames are the go test flags that make go rerun a test whose inputs did not change:
// -count with any value, and -shuffle, which go never caches, also as the test binary's own
// -test.count and -test.shuffle (after -args).
var rerunFlagNames = []string{"count", "shuffle", "test.count", "test.shuffle"}

// RerunFlag reports the first of args that makes go test rerun a test the cache already holds,
// with one dash or two, with its value after = or as the next argument.
func RerunFlag(args []string) (string, bool) {
	for _, arg := range args {
		if !strings.HasPrefix(arg, "-") {
			continue
		}
		name := strings.TrimLeft(arg, "-")
		if i := strings.IndexByte(name, '='); i >= 0 {
			name = name[:i]
		}
		if slices.Contains(rerunFlagNames, name) {
			return arg, true
		}
	}
	return "", false
}
