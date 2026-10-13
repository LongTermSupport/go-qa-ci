package root_test

import (
	"context"
	"os"
	"path/filepath"
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
