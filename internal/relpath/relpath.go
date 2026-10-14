// Package relpath maps the absolute paths a tool reports back to paths relative to a directory.
package relpath

import (
	"path/filepath"
	"strings"
)

// Within returns path relative to dir, slash-separated, and whether path lies inside dir. A dir
// that is a symbolic link also matches the paths reported under its target.
func Within(dir, path string) (string, bool) {
	bases := []string{dir}
	if resolved, err := filepath.EvalSymlinks(dir); err == nil && resolved != dir {
		bases = append(bases, resolved)
	}
	for _, base := range bases {
		rel, err := filepath.Rel(base, path)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return filepath.ToSlash(rel), true
		}
	}
	return "", false
}
