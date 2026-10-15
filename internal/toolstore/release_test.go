package toolstore

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// member is one file of a test archive.
type member struct{ name, body string }

func tarArchive(t *testing.T, mode int64, files []member) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, f := range files {
		name, body := f.name, f.body
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: mode, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestExtractWritesOnlyTheNamedMemberAsAnExecutable(t *testing.T) {
	// Even a member with a hostile name beside the wanted one is never written anywhere.
	archive := tarArchive(t, 0o644, []member{{"dir/tool", "binary"}, {"../../escape", "x"}})
	dest := filepath.Join(t.TempDir(), "tool")
	if err := extract(archive, false, "dir/tool", dest); err != nil {
		t.Fatal(err)
	}
	if got := readBack(t, dest); got != "binary" {
		t.Fatalf("content %q", got)
	}
	info, err := os.Stat(dest)
	if err != nil || info.Mode().Perm()&0o100 == 0 {
		t.Errorf("not executable by its owner: %v, %v", info, err)
	}
	if _, err = os.Stat(filepath.Join(filepath.Dir(dest), "..", "escape")); err == nil {
		t.Error("a member outside the wanted one was written")
	}
}

func TestExtractNamesAMissingMember(t *testing.T) {
	archive := tarArchive(t, 0o755, []member{{"other", "x"}})
	err := extract(archive, false, "dir/tool", filepath.Join(t.TempDir(), "tool"))
	if err == nil || !strings.Contains(err.Error(), "dir/tool") {
		t.Fatalf("got %v", err)
	}
}

func TestExtractRejectsWhatIsNotAnArchive(t *testing.T) {
	for _, zipped := range []bool{false, true} {
		if err := extract([]byte("not an archive"), zipped, "tool", filepath.Join(t.TempDir(), "tool")); err == nil {
			t.Errorf("zipped=%v: want an error", zipped)
		}
	}
}

func TestExtractReadsAZipMember(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("tool")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = w.Write([]byte("zipped")); err != nil {
		t.Fatal(err)
	}
	if err = zw.Close(); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "tool")
	if err = extract(buf.Bytes(), true, "tool", dest); err != nil {
		t.Fatal(err)
	}
	if got := readBack(t, dest); got != "zipped" {
		t.Fatalf("content %q", got)
	}
}

func TestEveryReleaseToolHasAnArchiveLayout(t *testing.T) {
	for name, spec := range releases {
		if !strings.HasPrefix(spec.url("1.2.3"), "https://") || spec.member("1.2.3") == "" {
			t.Errorf("%s: %q, %q", name, spec.url("1.2.3"), spec.member("1.2.3"))
		}
	}
}

// readBack reads the file extract wrote, through a root so the path cannot leave its directory.
func readBack(t *testing.T, dest string) string {
	t.Helper()
	root, err := os.OpenRoot(filepath.Dir(dest))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := root.Close(); closeErr != nil {
			t.Error(closeErr)
		}
	}()
	data, err := root.ReadFile(filepath.Base(dest))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
