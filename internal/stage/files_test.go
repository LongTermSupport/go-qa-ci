package stage_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

func write(t *testing.T, root, rel, text string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

// scripts writes the files the shell stage is exercised over.
func scripts(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, "scripts/build.sh", "echo build\n")
	write(t, root, "bin/run", "#!/usr/bin/env bash\necho run\n")
	write(t, root, "bin/flagged", "#!/usr/bin/env -S bash -e\necho run\n")
	write(t, root, "bin/direct", "#!/bin/sh\necho run\n")
	write(t, root, "bin/tool", "#!/usr/bin/python3\nprint()\n")
	write(t, root, "bin/plain", "no shebang\n")
	write(t, root, "vendor/lib.sh", "echo vendored\n")
	return root
}

func TestFilesAreChosenByGlobOrByShebangWithinTheScope(t *testing.T) {
	root := scripts(t)
	s := parse(t, shell)
	got, err := s.MatchingFiles(root, nil)
	want := []string{"bin/direct", "bin/flagged", "bin/run", "scripts/build.sh"}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("files = %v, %v; want %v", got, err, want)
	}
	scoped, err := s.MatchingFiles(root, []string{"scripts"})
	if err != nil || !reflect.DeepEqual(scoped, []string{"scripts/build.sh"}) {
		t.Errorf("scoped files = %v, %v", scoped, err)
	}
	whole, err := s.MatchingFiles(root, []string{"."})
	if err != nil || !reflect.DeepEqual(whole, want) {
		t.Errorf("a scope naming the root = %v, %v", whole, err)
	}
}

func TestNoMatchingFilesIsAnEmptyListNotAnError(t *testing.T) {
	root := t.TempDir()
	write(t, root, "main.go", "package main\n")
	got, err := parse(t, shell).MatchingFiles(root, nil)
	if err != nil || len(got) != 0 {
		t.Errorf("files = %v, %v", got, err)
	}
}

func TestAMalformedGlobIsAnError(t *testing.T) {
	s := parse(t, "name = \"x\"\nphase = \"runners\"\nsays = \"s\"\nfix = \"f\"\ncommand = [\"x\"]\nfiles = [\"[\"]\n")
	if _, err := s.MatchingFiles(t.TempDir(), nil); err == nil {
		t.Error("accepted")
	}
}

func git(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(context.Background(), "git", args...)
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestInsideARepositoryUntrackedFilesCountAndGitignoreIsHonoured(t *testing.T) {
	root := scripts(t)
	git(t, root, "init", "-q")
	write(t, root, ".gitignore", "scripts/ignored.sh\n")
	write(t, root, "scripts/ignored.sh", "echo ignored\n")
	git(t, root, "add", "scripts/build.sh")
	write(t, root, "scripts/untracked.sh", "echo untracked\n")
	write(t, root, "scripts/gone.sh", "echo gone\n")
	git(t, root, "add", "scripts/gone.sh")
	if err := os.Remove(filepath.Join(root, "scripts", "gone.sh")); err != nil {
		t.Fatal(err)
	}
	got, err := parse(t, shell).MatchingFiles(root, nil)
	want := []string{"bin/direct", "bin/flagged", "bin/run", "scripts/build.sh", "scripts/untracked.sh"}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("files = %v, %v; want %v", got, err, want)
	}
}

func TestOutsideARepositoryTheGitDirectoryIsStillSkipped(t *testing.T) {
	root := scripts(t)
	write(t, root, ".git/hooks/pre-commit.sh", "echo hook\n")
	got, err := parse(t, shell).MatchingFiles(root, nil)
	if err != nil || len(got) != 4 {
		t.Errorf("files = %v, %v", got, err)
	}
}

func TestSymbolicLinksAreNotChosen(t *testing.T) {
	root := scripts(t)
	if err := os.Symlink("build.sh", filepath.Join(root, "scripts", "link.sh")); err != nil {
		t.Fatal(err)
	}
	got, err := parse(t, shell).MatchingFiles(root, []string{"scripts"})
	if err != nil || !reflect.DeepEqual(got, []string{"scripts/build.sh"}) {
		t.Errorf("files = %v, %v", got, err)
	}
}
