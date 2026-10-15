package toolstore_test

import (
	"strings"
	"testing"

	goqaci "github.com/LongTermSupport/go-qa-ci"
	"github.com/LongTermSupport/go-qa-ci/internal/toolstore"
)

func TestParseLockReadsEverySection(t *testing.T) {
	lock, err := toolstore.ParseLock("# c\n[go]\nfoo = \"example.com/x/cmd/foo@v1.2.3\"\n[release]\nbar = \"4.5.6\"\n[checksums]\nbar = \"abc\"\n")
	if err != nil {
		t.Fatal(err)
	}
	foo, err := lock.Tool("foo")
	if err != nil || foo.Kind != toolstore.KindGo || foo.Version != "v1.2.3" || foo.Package != "example.com/x/cmd/foo" {
		t.Fatalf("foo: %+v, %v", foo, err)
	}
	bar, err := lock.Tool("bar")
	if err != nil || bar.Kind != toolstore.KindRelease || bar.Version != "4.5.6" || bar.SHA256 != "abc" {
		t.Fatalf("bar: %+v, %v", bar, err)
	}
	if got := strings.Join(lock.Names(), ","); got != "bar,foo" {
		t.Errorf("names: %s", got)
	}
}

func TestParseLockRejectsWhatItCannotRead(t *testing.T) {
	for name, text := range map[string]string{
		"unknown section": "[other]\na = \"b\"\n",
		"no section":      "a = \"b\"\n",
		"malformed line":  "[go]\nnot a pin\n",
		"go without @":    "[go]\nfoo = \"example.com/foo\"\n",
		"orphan checksum": "[release]\na = \"1\"\n[checksums]\nb = \"abc\"\n",
		"go checksum":     "[go]\nfoo = \"example.com/foo@v1\"\n[checksums]\nfoo = \"abc\"\n",
	} {
		if _, err := toolstore.ParseLock(text); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
}

func TestToolRefusesAnUnpinnedName(t *testing.T) {
	lock, err := toolstore.ParseLock("[go]\nfoo = \"example.com/foo@v1.0.0\"\n")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = lock.Tool("nope"); err == nil || !strings.Contains(err.Error(), "nope is not pinned in tools.lock") {
		t.Fatalf("got %v", err)
	}
}

func TestTheEmbeddedLockPinsEveryExternalTool(t *testing.T) {
	lock, err := toolstore.ParseLock(goqaci.ToolsLock)
	if err != nil {
		t.Fatal(err)
	}
	want := "actionlint,ast-grep,deadcode,gitleaks,golangci-lint,govulncheck,gremlins"
	if got := strings.Join(lock.Names(), ","); got != want {
		t.Errorf("embedded lock pins %s, want %s", got, want)
	}
}
