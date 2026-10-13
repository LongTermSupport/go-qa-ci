package glob_test

import (
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/glob"
)

func TestMatch(t *testing.T) {
	tests := []struct {
		pattern, name string
		want          bool
	}{
		{"src/a.go", "src/a.go", true},
		{"src/a.go", "src/b.go", false},
		{"*.go", "a.go", true},
		{"*.go", "dir/a.go", false},
		{"tests/**", "tests/x/y.go", true},
		{"tests/**", "tests", true},
		{"tests/**", "other/x.go", false},
		{"**/*.go", "a.go", true},
		{"**/*.go", "a/b/c.go", true},
		{"**/*.go", "a/b/c.txt", false},
		{"a/**/z.go", "a/z.go", true},
		{"a/**/z.go", "a/b/c/z.go", true},
		{"a/**/z.go", "a/b/c/y.go", false},
		{"src/[ab].go", "src/b.go", true},
		{"src/?.go", "src/ab.go", false},
	}
	for _, tc := range tests {
		got, err := glob.Match(tc.pattern, tc.name)
		if err != nil || got != tc.want {
			t.Errorf("Match(%q, %q) = %v, %v; want %v", tc.pattern, tc.name, got, err, tc.want)
		}
	}
}

func TestInvalidPatterns(t *testing.T) {
	for _, p := range []string{"src/[a.go", "a/[/b", "x\\"} {
		if err := glob.Valid(p); err == nil {
			t.Errorf("Valid(%q) succeeded", p)
		}
		if _, err := glob.Match(p, "src/a.go"); err == nil {
			t.Errorf("Match(%q) did not report the bad pattern", p)
		}
	}
}

func TestIsGlob(t *testing.T) {
	for p, want := range map[string]bool{"src/a.go": false, "src/*.go": true, "a/**": true, "a/?.go": true, "a/[b].go": true, "a{b,c}.go": false} {
		if got := glob.IsGlob(p); got != want {
			t.Errorf("IsGlob(%q) = %v, want %v", p, got, want)
		}
	}
}
