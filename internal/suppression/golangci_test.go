package suppression_test

import "testing"

// capsOff is the part of a configuration that keeps the issue caps off; routes are tested on top.
const capsOff = "issues:\n  max-issues-per-linter: 0\n  max-same-issues: 0\n"

const cfg = ".golangci.yml"

func TestGolangciRoutes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		yaml string
		want []string
	}{
		{"a clean configuration", "version: \"2\"\nlinters:\n  default: standard\n  enable: [gosec]\n" + capsOff, nil},
		{
			"exclusion rule", "linters:\n  exclusions:\n    rules:\n      - path: foo\\.go\n        linters: [gosec]\n        text: G306\n" + capsOff,
			[]string{"golangci.yml::exclusions.rules::gosec/G306@" + cfg + ":4"},
		},
		{
			"exclusion rule without linters or text", "linters:\n  exclusions:\n    rules:\n      - path: foo\\.go\n" + capsOff,
			[]string{"golangci.yml::exclusions.rules::*@" + cfg + ":4"},
		},
		{"exclusion paths", "linters:\n  exclusions:\n    paths:\n      - vendor\n" + capsOff, []string{"golangci.yml::exclusions.paths::vendor@" + cfg + ":4"}},
		{"exclusion presets", "linters:\n  exclusions:\n    presets: [comments]\n" + capsOff, []string{"golangci.yml::exclusions.presets::comments@" + cfg + ":3"}},
		{"generated files excluded", "linters:\n  exclusions:\n    generated: lax\n" + capsOff, []string{"golangci.yml::exclusions.generated::lax@" + cfg + ":3"}},
		{"generated files analysed", "linters:\n  exclusions:\n    generated: disable\n" + capsOff, nil},
		{
			"formatter exclusions", "formatters:\n  exclusions:\n    paths: [gen]\n    generated: strict\n" + capsOff,
			[]string{"golangci.yml::formatters.exclusions.paths::gen@" + cfg + ":3", "golangci.yml::formatters.exclusions.generated::strict@" + cfg + ":4"},
		},
		{
			"severity downgrade", "severity:\n  default: warning\n  rules:\n    - linters: [revive]\n      severity: info\n" + capsOff,
			[]string{"golangci.yml::severity.default::warning@" + cfg + ":2", "golangci.yml::severity.rules::revive/info@" + cfg + ":4"},
		},
		{"severity left at error", "severity:\n  default: error\n  rules:\n    - linters: [revive]\n      severity: error\n" + capsOff, nil},
		{"issue cap above zero", "issues:\n  max-issues-per-linter: 50\n  max-same-issues: 0\n", []string{"golangci.yml::issues.max-issues-per-linter::50@" + cfg + ":2"}},
		{"issue caps left unset: go-qa forces both to 0 on its own run", "linters:\n  default: standard\n", nil},
		{
			"new code only", "issues:\n  new: true\n  new-from-rev: HEAD~1\n  new-from-merge-base: main\n  new-from-patch: p.diff\n  max-issues-per-linter: 0\n  max-same-issues: 0\n",
			[]string{"golangci.yml::issues.new@" + cfg + ":2", "golangci.yml::issues.new-from-rev::HEAD~1@" + cfg + ":3", "golangci.yml::issues.new-from-merge-base::main@" + cfg + ":4", "golangci.yml::issues.new-from-patch::p.diff@" + cfg + ":5"},
		},
		{"new switched off", "issues:\n  new: false\n  max-issues-per-linter: 0\n  max-same-issues: 0\n", nil},
		{"tests not analysed", "run:\n  tests: false\n" + capsOff, []string{"golangci.yml::run.tests@" + cfg + ":2"}},
		{"tests analysed", "run:\n  tests: true\n" + capsOff, nil},
		{"linters disabled is choosing the defence set, not a suppression", "linters:\n  default: all\n  disable: [errcheck, unused, wsl]\n" + capsOff, nil},
		{"linters enabled and disabled beside a real exclusion", "linters:\n  disable: [wsl]\n  exclusions:\n    presets: [comments]\n" + capsOff, []string{"golangci.yml::exclusions.presets::comments@" + cfg + ":4"}},
		{"a reduced default set", "linters:\n  default: none\n" + capsOff, []string{"golangci.yml::linters.default::none@" + cfg + ":2"}},
		{"every linter", "linters:\n  default: all\n" + capsOff, nil},
		{"errcheck excludes", "linters:\n  settings:\n    errcheck:\n      exclude-functions: [fmt.Println]\n" + capsOff, []string{"golangci.yml::errcheck.exclude-functions::fmt.Println@" + cfg + ":4"}},
		{"gosec excludes", "linters:\n  settings:\n    gosec:\n      excludes: [G204, G306]\n" + capsOff, []string{"golangci.yml::gosec.excludes::G204@" + cfg + ":4", "golangci.yml::gosec.excludes::G306@" + cfg + ":4"}},
		{
			"govet disable", "linters:\n  settings:\n    govet:\n      disable: [shadow]\n      disable-all: true\n" + capsOff,
			[]string{"golangci.yml::govet.disable::shadow@" + cfg + ":4", "golangci.yml::govet.disable-all@" + cfg + ":5"},
		},
		{
			"gocritic disabled checks", "linters:\n  settings:\n    gocritic:\n      disabled-checks: [x]\n      disabled-tags: [y]\n" + capsOff,
			[]string{"golangci.yml::gocritic.disabled-checks::x@" + cfg + ":4", "golangci.yml::gocritic.disabled-tags::y@" + cfg + ":5"},
		},
		{"staticcheck check removals", "linters:\n  settings:\n    staticcheck:\n      checks: [all, -SA1019]\n" + capsOff, []string{"golangci.yml::staticcheck.checks::SA1019@" + cfg + ":4"}},
		{
			"testifylint disable", "linters:\n  settings:\n    testifylint:\n      disable: [a]\n      disable-all: true\n" + capsOff,
			[]string{"golangci.yml::testifylint.disable::a@" + cfg + ":4", "golangci.yml::testifylint.disable-all@" + cfg + ":5"},
		},
		{
			"revive rules", "linters:\n  settings:\n    revive:\n      ignore-generated-header: true\n      rules:\n        - name: exported\n          disabled: true\n        - name: var-naming\n          exclude: [foo.go]\n        - name: kept\n" + capsOff,
			[]string{"golangci.yml::revive.ignore-generated-header@" + cfg + ":4", "golangci.yml::revive.rules::exported/disabled@" + cfg + ":7", "golangci.yml::revive.rules::var-naming/exclude@" + cfg + ":9"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			wantExactly(t, ruleLines(scan(t, map[string]string{cfg: tc.yaml})), tc.want...)
		})
	}
}

func TestGolangciConfigFormats(t *testing.T) {
	t.Parallel()
	json := "{\"linters\": {\"exclusions\": {\"presets\": [\"comments\"]}}, \"issues\": {\"max-issues-per-linter\": 0, \"max-same-issues\": 0}}"
	wantExactly(t, ruleLines(scan(t, map[string]string{"sub/.golangci.json": json})), "golangci.yml::exclusions.presets::comments@sub/.golangci.json:1")
	toml := "[linters.exclusions]\npresets = [\"comments\"]\n[issues]\nmax-issues-per-linter = 0\nmax-same-issues = 0\n"
	wantExactly(t, ruleLines(scan(t, map[string]string{".golangci.toml": toml})), "golangci.yml::exclusions.presets::comments@.golangci.toml:0")
	wantExactly(t, ruleLines(scan(t, map[string]string{".golangci.yaml": "linters:\n  exclusions:\n    presets: [comments]\n" + capsOff})), "golangci.yml::exclusions.presets::comments@.golangci.yaml:3")
}

func TestUnparseableGolangciConfigIsReported(t *testing.T) {
	t.Parallel()
	result := scan(t, map[string]string{cfg: "linters: [unclosed\n", ".golangci.toml": "linters = ["})
	if len(result.Unparseable) != 2 {
		t.Fatalf("got %+v", result.Unparseable)
	}
}
