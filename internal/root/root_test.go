package root_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/root"
)

func TestDiscoverFindsModuleRoot(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/m\n\ngo 1.26\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(dir, "pkg", "inner")
	if err := os.MkdirAll(sub, 0o750); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOWORK", "off")
	got, err := root.Discover(context.Background(), sub)
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("root = %q, want %q", got, want)
	}
}

func TestDiscoverPrefersWorkspace(t *testing.T) {
	dir := t.TempDir()
	mod := filepath.Join(dir, "mod")
	if err := os.MkdirAll(mod, 0o750); err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string]string{
		filepath.Join(mod, "go.mod"):  "module example.com/m\n\ngo 1.26\n",
		filepath.Join(dir, "go.work"): "go 1.26\n\nuse ./mod\n",
	} {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("GOWORK", "")
	got, err := root.Discover(context.Background(), mod)
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("root = %q, want the go.work directory %q", got, want)
	}
}

func TestDiscoverOutsideAModuleFails(t *testing.T) {
	t.Setenv("GOWORK", "off")
	if _, err := root.Discover(context.Background(), t.TempDir()); err == nil {
		t.Error("a directory outside any module must be an error")
	}
}

func TestRelativeScopesAPathToTheRoot(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "pkg"), 0o750); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "pkg", "a.go")
	if err := os.WriteFile(file, []byte("package pkg\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	other := t.TempDir()
	tests := []struct {
		name    string
		path    string
		want    string
		wantErr string
	}{
		{"absolute file", file, "pkg/a.go", ""},
		{"directory", filepath.Join(dir, "pkg"), "pkg", ""},
		{"the root itself", dir, ".", ""},
		{"uncleaned", filepath.Join(dir, "pkg", "..", "pkg", "a.go"), "pkg/a.go", ""},
		{"outside the root", other, "", "outside"},
		{"missing", filepath.Join(dir, "nope.go"), "", "nope.go"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := root.Relative(dir, tc.path)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want containing %q", err, tc.wantErr)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Errorf("Relative = %q, %v; want %q", got, err, tc.want)
			}
		})
	}
}

func TestRelativeResolvesRelativeToTheWorkingDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	got, err := root.Relative(dir, "a.go")
	if err != nil || got != "a.go" {
		t.Errorf("Relative = %q, %v", got, err)
	}
}
