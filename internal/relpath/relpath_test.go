package relpath_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/relpath"
)

func TestWithinGivesASlashSeparatedPathRelativeToTheDirectory(t *testing.T) {
	dir := t.TempDir()
	got, ok := relpath.Within(dir, filepath.Join(dir, "a", "b.go"))
	if !ok || got != "a/b.go" {
		t.Errorf("got %q, %v", got, ok)
	}
}

func TestWithinRefusesAPathOutsideTheDirectory(t *testing.T) {
	dir := t.TempDir()
	for _, outside := range []string{filepath.Dir(dir), filepath.Join(filepath.Dir(dir), "other", "x.go"), dir + "-sibling/x.go"} {
		if got, ok := relpath.Within(dir, outside); ok {
			t.Errorf("%s was reported inside as %q", outside, got)
		}
	}
}

func TestWithinResolvesASymlinkedDirectory(t *testing.T) {
	target := t.TempDir()
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	got, ok := relpath.Within(link, filepath.Join(target, "x.go"))
	if !ok || got != "x.go" {
		t.Errorf("got %q, %v", got, ok)
	}
}
