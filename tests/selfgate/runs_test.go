package selfgate_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/astgrep"
	"github.com/LongTermSupport/go-qa-ci/internal/config"
	"github.com/LongTermSupport/go-qa-ci/internal/tools"
)

// selfGateTools are the tools proven active in the self-gate: the formatter, the golangci-lint,
// suppression, analyzers and deadcode detectors, and the vuln, mod, test and coverage runners.
var selfGateTools = []string{"fmt", "golangci", "suppression", "analyzers", "deadcode", "vuln", "mod", "test", "coverage"}

// selfGateToolsFor is the set of tools the self-gate must run: the fixed set, plus ast-grep as soon
// as there is a rule for it to run, a bundled one or a project's own. Until then the step is
// skipped saying so, and a skipped step proves nothing; from then on a switched-off or missing
// ast-grep must fail the build, not pass quietly.
func selfGateToolsFor(packEmpty, hasProjectRules bool) []string {
	if packEmpty && !hasProjectRules {
		return selfGateTools
	}
	return append(slices.Clone(selfGateTools), "ast-grep")
}

// disabledSelfGateTools names the proven tools that a plain `go-qa run` would not execute.
func disabledSelfGateTools(cfg config.Tools, required []string) []string {
	var off []string
	for _, name := range required {
		tool, ok := tools.FromName(name)
		if !ok {
			off = append(off, name+" (not in the catalogue)")
			continue
		}
		if !tool.Enabled(cfg) {
			off = append(off, name)
		}
	}
	return off
}

func loadOwnConfig(t *testing.T) config.Config {
	t.Helper()
	cfg, err := config.Load(filepath.Join("..", ".."), "")
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

// The self-gate analyses the toolchain with the tools the configuration leaves on, so switching
// any proven tool off in qa/go-qa.toml must fail the build.
func TestSelfGateRunsTheFormatterAndGolangci(t *testing.T) {
	t.Parallel()
	cfg := loadOwnConfig(t)
	pack, err := astgrep.Bundled()
	if err != nil {
		t.Fatal(err)
	}
	_, hasSgconfig := os.Stat(filepath.Join("..", "..", "sgconfig.yml"))
	required := selfGateToolsFor(pack.Empty() || !cfg.AstGrep.Bundled, hasSgconfig == nil)
	if off := disabledSelfGateTools(cfg.Tools, required); len(off) != 0 {
		t.Fatalf("qa/go-qa.toml switches off %v; the self-gate would not run them (TOOLING-SPEC 8.2)", off)
	}
}

func TestSelfGateMustRunAstGrepOnceThereIsARuleForItToRun(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		packEmpty, project, want bool
	}{
		"nothing to run":     {true, false, false},
		"the pack has rules": {false, true, true},
		"a project rule":     {true, true, true},
		"the pack alone":     {false, false, true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := slices.Contains(selfGateToolsFor(tc.packEmpty, tc.project), "ast-grep")
			if got != tc.want {
				t.Errorf("ast-grep required = %v, want %v", got, tc.want)
			}
		})
	}
	cfg := config.Default().Tools
	cfg.AstGrep = false
	if off := disabledSelfGateTools(cfg, selfGateToolsFor(false, false)); !slices.Contains(off, "ast-grep") {
		t.Errorf("a switched-off ast-grep went unnoticed once the pack has rules: %v", off)
	}
}

func TestSelfGateToolCheckDetectsASwitchedOffTool(t *testing.T) {
	t.Parallel()
	for _, name := range selfGateTools {
		cfg := config.Default().Tools
		switch name {
		case "fmt":
			cfg.Fmt = false
		case "golangci":
			cfg.Golangci = false
		case "suppression":
			cfg.Suppression = false
		case "analyzers":
			cfg.Analyzers = false
		case "deadcode":
			cfg.Deadcode = false
		case "vuln":
			cfg.Vuln = false
		case "mod":
			cfg.Mod = false
		case "test":
			cfg.Test = false
		case "coverage":
			cfg.Coverage = false
		}
		if off := disabledSelfGateTools(cfg, selfGateTools); !slices.Equal(off, []string{name}) {
			t.Errorf("switching %s off reported %v", name, off)
		}
	}
}

// selfGateInvokesCI reports whether some job has a step that runs `go-qa run --ci`.
func selfGateInvokesCI(workflow map[string]any) bool {
	jobs, ok := workflow["jobs"].(map[string]any)
	if !ok {
		return false
	}
	for _, job := range jobs {
		jobMap, isMap := job.(map[string]any)
		if !isMap {
			continue
		}
		steps, isList := jobMap["steps"].([]any)
		if !isList {
			continue
		}
		for _, step := range steps {
			stepMap, isStep := step.(map[string]any)
			if !isStep {
				continue
			}
			if run, isRun := stepMap["run"].(string); isRun && runsCI(run) {
				return true
			}
		}
	}
	return false
}

// runsCI matches a script line that executes go-qa with exactly `run --ci` as its arguments.
func runsCI(script string) bool {
	for line := range strings.SplitSeq(script, "\n") {
		fields := strings.Fields(line)
		for i, field := range fields {
			if strings.HasSuffix(strings.Trim(field, `"`), "go-qa") && slices.Equal(fields[i+1:], []string{"run", "--ci"}) {
				return true
			}
		}
	}
	return false
}

func TestCIWorkflowSelfGateJobRunsGoQaRunCI(t *testing.T) {
	t.Parallel()
	workflow := readYAML(t, repoPath(".github", "workflows", "ci.yml"))
	if !selfGateInvokesCI(workflow) {
		t.Fatal(".github/workflows/ci.yml has no step that runs `go-qa run --ci` (TOOLING-SPEC 8.2)")
	}
}

func TestWorkflowCheckDetectsAMissingSelfGate(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"no job":         "jobs: {}\n",
		"other command":  "jobs:\n  a:\n    steps:\n      - run: go-qa run\n",
		"other argument": "jobs:\n  a:\n    steps:\n      - run: go-qa list --ci\n",
		"not a workflow": "name: ci\n",
	}
	for name, content := range cases {
		if selfGateInvokesCI(decode(t, []byte(content), name)) {
			t.Errorf("%s: accepted as a self-gate", name)
		}
	}
	accepted := "jobs:\n  a:\n    steps:\n      - run: |\n          \"${RUNNER_TEMP}/bin/go-qa\" run --ci\n"
	if !selfGateInvokesCI(decode(t, []byte(accepted), "accepted")) {
		t.Error("a step running go-qa run --ci was rejected")
	}
}
