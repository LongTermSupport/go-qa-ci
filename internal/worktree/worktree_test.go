package worktree_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/worktree"
)

func write(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestChangedReportsModifiedAddedAndRemovedGoFiles(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "keep.go"), "package a\n")
	write(t, filepath.Join(dir, "edit.go"), "package a\n")
	write(t, filepath.Join(dir, "sub", "gone.go"), "package a\n")
	write(t, filepath.Join(dir, "notes.txt"), "text\n")
	write(t, filepath.Join(dir, ".git", "hook.go"), "package a\n")
	before, err := worktree.Snapshot(dir)
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, "edit.go"), "package b\n")
	write(t, filepath.Join(dir, "new.go"), "package a\n")
	write(t, filepath.Join(dir, "notes.txt"), "changed\n")
	write(t, filepath.Join(dir, ".git", "hook.go"), "package b\n")
	if removeErr := os.Remove(filepath.Join(dir, "sub", "gone.go")); removeErr != nil {
		t.Fatal(removeErr)
	}
	after, err := worktree.Snapshot(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"edit.go", "new.go", "sub/gone.go"}
	if got := worktree.Changed(before, after); !slices.Equal(got, want) {
		t.Errorf("Changed = %v, want %v", got, want)
	}
	if got := worktree.Changed(after, after); len(got) != 0 {
		t.Errorf("unchanged tree reported %v", got)
	}
}
