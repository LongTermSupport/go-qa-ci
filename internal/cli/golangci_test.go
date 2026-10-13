package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/cli"
)

func writeModule(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	files["go.mod"] = "module example.com/fixture\n\ngo 1.26\n"
	for name, text := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func goQA(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code = cli.Run(args, &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestRunCIFailsOnAnUnformattedModuleBecauseTheTreeChanged(t *testing.T) {
	root := writeModule(t, map[string]string{
		"p.go": "// Package p is a fixture.\npackage p\n\n// F is a fixture.\nfunc F() {\n\tprintln( \"hi\")\n}\n",
	})
	code, stdout, stderr := goQA(t, "--root", root, "run", "-t", "fmt", "--ci")
	if code != cli.ExitFailure || !strings.Contains(stderr, "working tree changed") {
		t.Errorf("exit %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "fmt") || !strings.Contains(stdout, "FAILED") {
		t.Errorf("summary does not show the failure:\n%s", stdout)
	}
}

func TestRunReportsAnErrcheckViolationWithItsIdentifier(t *testing.T) {
	root := writeModule(t, map[string]string{
		"p.go": "// Package p is a fixture.\npackage p\n\nimport \"os\"\n\n// F is a fixture.\nfunc F() {\n\tos.Remove(\"x\")\n}\n",
	})
	code, _, stderr := goQA(t, "--root", root, "run", "-t", "golangci")
	if code != cli.ExitFailure || !strings.Contains(stderr, "error[errcheck]") {
		t.Errorf("exit %d\nstderr:\n%s", code, stderr)
	}
}

func TestRunDropsAFindingOutsideThePathScope(t *testing.T) {
	root := writeModule(t, map[string]string{
		"p/p.go": "// Package p is a fixture.\npackage p\n\nimport \"os\"\n\n// F is a fixture.\nfunc F() {\n\tos.Remove(\"x\")\n}\n",
		"q/q.go": "// Package q is a fixture.\npackage q\n",
	})
	code, _, stderr := goQA(t, "--root", root, "run", "-t", "golangci", "-p", filepath.Join(root, "q"))
	if code != cli.ExitOK || stderr != "" {
		t.Errorf("a finding outside -p was reported: exit %d\n%s", code, stderr)
	}
}
