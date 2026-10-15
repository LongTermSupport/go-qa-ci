package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/cli"
)

func module(t *testing.T, config string) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{"go.mod": "module example.com/m\n\ngo 1.26\n"}
	if config != "" {
		files["qa/go-qa.toml"] = config
	}
	for rel, content := range files {
		path := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func run(args ...string) (code int, stdout, stderr string) {
	var out, errOut bytes.Buffer
	code = cli.Run(args, &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestBasics(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantOut    string
		wantErrSub string
	}{
		{"version", []string{"--version"}, 0, cli.Version + "\n", ""},
		{"unknown subcommand", []string{"bogus"}, 2, "", "unknown subcommand"},
		{"no arguments", nil, 2, "", "usage"},
		{"unknown global flag", []string{"--nope", "tools"}, 2, "", "nope"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			code, out, errOut := run(tc.args...)
			if code != tc.wantCode || out != tc.wantOut || !strings.Contains(errOut, tc.wantErrSub) {
				t.Errorf("got %d, %q, %q; want %d, %q, stderr containing %q", code, out, errOut, tc.wantCode, tc.wantOut, tc.wantErrSub)
			}
		})
	}
}

const defaultTools = `fmt          format     enabled
record       detectors  enabled
suppression  detectors  enabled
summary      detectors  enabled
docs         detectors  enabled
ast-grep     detectors  enabled
golangci     detectors  enabled
analyzers    detectors  enabled
deadcode     detectors  enabled
vuln         runners    enabled
mod          runners    enabled
build        runners    enabled
test         runners    enabled
coverage     runners    enabled
mutants      runners    enabled
`

func TestToolsListsCatalogueInPhaseOrder(t *testing.T) {
	code, out, errOut := run("--root", module(t, ""), "tools")
	if code != 0 || errOut != "" {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	if out != defaultTools {
		t.Errorf("tools output:\n%s\nwant:\n%s", out, defaultTools)
	}
}

func TestToolsShowsDisabled(t *testing.T) {
	root := module(t, "[tools]\nvuln = false\n")
	_, out, _ := run("--root", root, "tools")
	if !strings.Contains(out, "vuln         runners    disabled\n") {
		t.Errorf("vuln not shown disabled:\n%s", out)
	}
}

func TestUnknownConfigKeyExitsTwoNamingKey(t *testing.T) {
	root := module(t, "[tools]\nbogus_tool = true\n")
	code, out, errOut := run("--root", root, "tools")
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
	if out != "" {
		t.Errorf("stdout = %q, want empty", out)
	}
	if !strings.Contains(errOut, "tools.bogus_tool") {
		t.Errorf("stderr %q does not name the key", errOut)
	}
}

func TestConfigFlag(t *testing.T) {
	root := module(t, "")
	if err := os.WriteFile(filepath.Join(root, "alt.toml"), []byte("[tools]\nmutants = false\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, out, _ := run("--root", root, "--config", "alt.toml", "tools")
	if !strings.Contains(out, "mutants      runners    disabled\n") {
		t.Errorf("--config ignored:\n%s", out)
	}
	if code, _, _ := run("--root", root, "--config", "nope.toml", "tools"); code != 2 {
		t.Errorf("missing --config file exit = %d, want 2", code)
	}
}

func TestToolsRejectsExtraArguments(t *testing.T) {
	if code, _, _ := run("--root", module(t, ""), "tools", "extra"); code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
}
