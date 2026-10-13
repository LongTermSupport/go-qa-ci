// Package glob matches slash-separated relative paths against patterns: path.Match syntax per
// segment, plus a `**` segment that matches any number of whole segments (including none).
package glob

import (
	"fmt"
	"path"
	"strings"
)

const anyDepth = "**"

// IsGlob reports whether pattern uses wildcard syntax rather than naming one path.
func IsGlob(pattern string) bool {
	return strings.ContainsAny(pattern, "*?[")
}

// Valid reports why pattern is malformed, or nil.
func Valid(pattern string) error {
	for _, segment := range strings.Split(pattern, "/") {
		if segment == anyDepth {
			continue
		}
		// An empty name never matches a non-empty segment, but path.Match still rejects a
		// malformed pattern while scanning it.
		if _, err := path.Match(segment, ""); err != nil {
			return fmt.Errorf("invalid glob %q: %w", pattern, err)
		}
	}
	return nil
}

// Match reports whether name matches pattern; it fails when the pattern is malformed.
func Match(pattern, name string) (bool, error) {
	if err := Valid(pattern); err != nil {
		return false, err
	}
	return matchSegments(strings.Split(pattern, "/"), strings.Split(name, "/")), nil
}

func matchSegments(pattern, name []string) bool {
	for len(pattern) > 0 {
		if pattern[0] == anyDepth {
			rest := pattern[1:]
			for skip := 0; skip <= len(name); skip++ {
				if matchSegments(rest, name[skip:]) {
					return true
				}
			}
			return false
		}
		if len(name) == 0 {
			return false
		}
		if ok, err := path.Match(pattern[0], name[0]); err != nil || !ok {
			return false
		}
		pattern, name = pattern[1:], name[1:]
	}
	return len(name) == 0
}
