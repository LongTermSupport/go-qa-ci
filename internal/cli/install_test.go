package cli_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/cli"
	"github.com/LongTermSupport/go-qa-ci/internal/pipeline"
	"github.com/LongTermSupport/go-qa-ci/internal/tools"
)

// fakeChain is a Toolchain that records what it was asked for, installs into a set and fails on
// demand; it never touches the network.
type fakeChain struct {
	pinned    []string
	installed map[string]bool
	ensured   [][]string
	installs  [][]string
	failWith  error
	events    *[]string
}

func newChain(pinned ...string) *fakeChain {
	return &fakeChain{pinned: pinned, installed: map[string]bool{}, events: &[]string{}}
}

func (c *fakeChain) Ensure(_ context.Context, names ...string) error {
	*c.events = append(*c.events, "ensure")
	c.ensured = append(c.ensured, names)
	if c.failWith != nil {
		return c.failWith
	}
	for _, n := range names {
		c.installed[n] = true
	}
	return nil
}

func (c *fakeChain) Install(_ context.Context, names ...string) error {
	c.installs = append(c.installs, names)
	if c.failWith != nil {
		return c.failWith
	}
	if len(names) == 0 {
		names = c.pinned
	}
	for _, n := range names {
		c.installed[n] = true
	}
	return nil
}

func (c *fakeChain) Missing() []string {
	var missing []string
	for _, n := range c.pinned {
		if !c.installed[n] {
			missing = append(missing, n)
		}
	}
	return missing
}

func (c *fakeChain) Resolve(name string) (string, bool) {
	if !c.installed[name] {
		return "", false
	}
	return "/tools/" + name + "/v1/" + name, true
}

func (c *fakeChain) Names() []string { return c.pinned }

func runChain(chain *fakeChain, rec *recorder, args ...string) (code int, stdout, stderr string) {
	var out, errOut bytes.Buffer
	factory := func(io.Writer) (cli.Toolchain, error) { return chain, nil }
	if rec == nil {
		rec = &recorder{}
	}
	code = cli.RunWithToolchain(args, &out, &errOut, func(ctx tools.Context, w io.Writer) pipeline.Executor {
		*chain.events = append(*chain.events, "executor")
		return rec.factory(ctx, w)
	}, factory)
	return code, out.String(), errOut.String()
}

func TestRunInstallsTheToolsItsStepsNeedBeforeTheGate(t *testing.T) {
	chain := newChain("golangci-lint", "ast-grep", "deadcode", "govulncheck", "gremlins", "gitleaks", "actionlint")
	rec := &recorder{}
	code, _, errOut := runChain(chain, rec, "--root", module(t, "[tools]\nmutants = false\n"), "run", "-t", "ast-grep", "-t", "deadcode", "-t", "golangci")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	want := []string{"ast-grep", "deadcode", "golangci-lint"}
	if len(chain.ensured) != 1 || !reflect.DeepEqual(chain.ensured[0], want) {
		t.Errorf("ensured %v, want one call with %v", chain.ensured, want)
	}
	if got := strings.Join(*chain.events, ","); got != "ensure,executor" {
		t.Errorf("order %s: the tools must be installed before the gate runs", got)
	}
}

func TestRunDoesNotInstallForStepsThatNeedNoTool(t *testing.T) {
	chain := newChain("golangci-lint")
	code, _, errOut := runChain(chain, nil, "--root", module(t, ""), "run", "-t", "record", "-t", "test")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	if len(chain.ensured) != 0 {
		t.Errorf("ensured %v: an empty list means every pinned tool to the store, so no call must be made", chain.ensured)
	}
}

func TestRunStopsWhenAPinnedToolCannotBeInstalled(t *testing.T) {
	chain := newChain("ast-grep")
	chain.failWith = errors.New("sha256 mismatch")
	rec := &recorder{}
	code, out, errOut := runChain(chain, rec, "--root", module(t, ""), "run", "-t", "ast-grep")
	if code != cli.ExitFailure || !strings.Contains(errOut, "sha256 mismatch") {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	if len(rec.ran) != 0 || strings.Contains(out, "passed") {
		t.Errorf("the gate ran without its tool: %v", rec.ran)
	}
}

func TestInstallToolsInstallsEverythingAndPrintsNothingElseOnStdout(t *testing.T) {
	chain := newChain("a", "b")
	code, out, errOut := runChain(chain, nil, "install-tools")
	if code != 0 || out != "" {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, out, errOut)
	}
	if len(chain.installs) != 1 || len(chain.installs[0]) != 0 {
		t.Errorf("installs %v, want one call for every tool", chain.installs)
	}
}

func TestInstallToolsInstallsTheNamedTools(t *testing.T) {
	chain := newChain("a", "b")
	if code, _, errOut := runChain(chain, nil, "install-tools", "b"); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	if !reflect.DeepEqual(chain.installs, [][]string{{"b"}}) {
		t.Errorf("installs %v", chain.installs)
	}
}

func TestInstallToolsCheckReportsMissingToolsAndInstallsNothing(t *testing.T) {
	chain := newChain("a", "b")
	chain.installed["a"] = true
	code, _, errOut := runChain(chain, nil, "install-tools", "--check")
	if code != cli.ExitFailure || !strings.Contains(errOut, "b") || strings.Contains(errOut, "a is") {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	if len(chain.installs) != 0 {
		t.Errorf("--check installed: %v", chain.installs)
	}
	chain.installed["b"] = true
	if code, _, _ = runChain(chain, nil, "install-tools", "--check"); code != 0 {
		t.Errorf("exit %d with nothing missing", code)
	}
}

func TestInstallToolsPrintPathListsTheDirectoryOfEachTool(t *testing.T) {
	chain := newChain("a", "b")
	code, out, errOut := runChain(chain, nil, "install-tools", "--print-path")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	if out != "/tools/a/v1\n/tools/b/v1\n" {
		t.Errorf("stdout %q", out)
	}
}

func TestInstallToolsFailsWithTheCauseWhenAnInstallFails(t *testing.T) {
	chain := newChain("a")
	chain.failWith = errors.New("network down")
	code, _, errOut := runChain(chain, nil, "install-tools")
	if code != cli.ExitFailure || !strings.Contains(errOut, "network down") {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
}

func TestInstallToolsRejectsAnUnknownFlag(t *testing.T) {
	chain := newChain("a")
	if code, _, _ := runChain(chain, nil, "install-tools", "--nope"); code != cli.ExitUsage {
		t.Errorf("exit %d, want usage", code)
	}
}

func TestInstallToolsNeedsNoModule(t *testing.T) {
	// A container image build runs it with no go.mod anywhere near.
	chain := newChain("a")
	t.Chdir(t.TempDir())
	if code, _, errOut := runChain(chain, nil, "install-tools"); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
}
