package selfgate_test

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/config"
	"github.com/LongTermSupport/go-qa-ci/internal/tools"
)

// selfGateTools are the tools whose rules are proven active: the formatter, the golangci-lint
// detector, the suppression detector and the analyzers detector.
var selfGateTools = []string{"fmt", "golangci", "suppression", "analyzers"}

// disabledSelfGateTools names the proven tools that a plain `go-qa run` would not execute.
func disabledSelfGateTools(cfg config.Tools) []string {
	var off []string
	for _, name := range selfGateTools {
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
// fmt, golangci or suppression off in qa/go-qa.toml must fail the build.
func TestSelfGateRunsTheFormatterAndGolangci(t *testing.T) {
	t.Parallel()
	if off := disabledSelfGateTools(loadOwnConfig(t).Tools); len(off) != 0 {
		t.Fatalf("qa/go-qa.toml switches off %v; the self-gate would not run them (TOOLING-SPEC 8.2)", off)
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
		}
		if off := disabledSelfGateTools(cfg); !slices.Equal(off, []string{name}) {
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
