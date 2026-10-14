package config_test

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/config"
)

func writeFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	path := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// readmeDefaults returns the toml blocks of the README "Configuration" section.
func readmeDefaults(t *testing.T) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	_, section, found := strings.Cut(string(data), "\n## Configuration\n")
	if !found {
		t.Fatal("README has no Configuration section")
	}
	if next := strings.Index(section, "\n## "); next >= 0 {
		section = section[:next]
	}
	var blocks []string
	for _, m := range regexp.MustCompile("(?s)```toml\n(.*?)```").FindAllStringSubmatch(section, -1) {
		blocks = append(blocks, m[1])
	}
	if len(blocks) < 2 {
		t.Fatalf("README Configuration section has %d toml blocks, want at least 2", len(blocks))
	}
	return blocks
}

func TestReadmeDefaultsMatchBuiltInDefaults(t *testing.T) {
	for i, block := range readmeDefaults(t) {
		dir := t.TempDir()
		writeFile(t, dir, config.DefaultPath, block)
		got, err := config.Load(dir, "")
		if err != nil {
			t.Fatalf("block %d: %v", i, err)
		}
		if want := config.Default(); !reflect.DeepEqual(got, want) {
			t.Errorf("block %d decodes to\n%+v\nwant the built-in defaults\n%+v", i, got, want)
		}
	}
}

func TestLoadWithoutFileGivesDefaults(t *testing.T) {
	got, err := config.Load(t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	if want := config.Default(); !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want defaults", got)
	}
}

func TestLoadOverridesOnlyWhatIsSet(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, config.DefaultPath, "[tools]\ndeadcode = false\ntest = false\n[coverage]\nfail_under_statements = 90\n")
	got, err := config.Load(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	want := config.Default()
	want.Tools.Deadcode = false
	want.Tools.Test = false
	want.Coverage.FailUnderStatements = 90
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestRecordShorthandAndTable(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, config.DefaultPath, `record = "qa/other.toml"`)
	got, err := config.Load(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	want := config.Default().Record
	want.Path = "qa/other.toml"
	if !reflect.DeepEqual(got.Record, want) {
		t.Errorf("shorthand: got %+v, want %+v", got.Record, want)
	}

	writeFile(t, dir, config.DefaultPath, "[record]\nmax_total = 4\nmax_per_rule = 2\nmax_policy = 1\ndeciders = [\"example-decider\"]\n[record.trust]\nmodels = { opus = \"high\" }\n[[record.trust.cosign]]\nrules = [\"gosec::*\"]\nmin_tier = \"high\"\n")
	got, err = config.Load(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	if got.Record.MaxTotal != 4 || got.Record.MaxPolicy != 1 || got.Record.Deciders[0] != "example-decider" ||
		got.Record.Trust.Models["opus"] != config.TierHigh ||
		got.Record.Trust.Cosign[0].MinTier != config.TierHigh {
		t.Errorf("table decoded wrongly: %+v", got.Record)
	}
}

func TestLoadRejectsBadConfig(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantSub []string
	}{
		{"unknown top-level key", "bogus = 1\n", []string{"bogus"}},
		{"unknown tools key", "[tools]\nfmt = true\nbogus = true\n", []string{"tools.bogus"}},
		{"unknown record key", "[record]\nmax_totl = 3\n", []string{"record.max_totl"}},
		{"unknown record key via shorthand table", "[record.trust.high]\nmax_days = 3\n", []string{"record.trust.high.max_days"}},
		{"several unknown keys all named", "one = 1\n[tools]\ntwo = 2\n", []string{"one", "tools.two"}},
		{"unknown mutants mode", "[mutants]\nmode = \"sometimes\"\n", []string{"mutants", "sometimes"}},
		{"unknown tier", "[record.trust]\nmodels = { x = \"cosmic\" }\n", []string{"cosmic"}},
		{"record of wrong type", "record = 5\n", []string{"record", "path or a table"}},
		{"coverage above 100", "[coverage]\nfail_under_statements = 101\n", []string{"fail_under_statements", "100"}},
		{"efficacy above 100", "[mutants]\nthreshold_efficacy = 101\n", []string{"threshold_efficacy", "100"}},
		{"efficacy negative", "[mutants]\nthreshold_efficacy = -1\n", []string{"threshold_efficacy"}},
		{"coverage negative", "[coverage]\nfail_under_statements = -1\n", []string{"fail_under_statements"}},
		{"max_per_rule above max_total", "[record]\nmax_total = 2\nmax_per_rule = 3\n", []string{"max_per_rule", "max_total"}},
		{"max_policy above max_per_rule", "[record]\nmax_policy = 4\n", []string{"max_policy", "max_per_rule"}},
		{"absolute docs_dir", "docs_dir = \"/etc\"\n", []string{"docs_dir"}},
		{"escaping record path", "record = \"../record.toml\"\n", []string{"record.path"}},
		{"module without go.mod", "modules = [\"nested\"]\n", []string{"nested", "go.mod"}},
		{"absolute analyzers driver", "[analyzers]\ndriver = \"/usr/lib/driver\"\n", []string{"analyzers.driver"}},
		{"escaping analyzers driver", "[analyzers]\ndriver = \"../driver\"\n", []string{"analyzers.driver"}},
		{"analyzers driver not a relative package", "[analyzers]\ndriver = \"example.com/other/driver\"\n", []string{"analyzers.driver", "./"}},
		{"unknown analyzers key", "[analyzers]\ndrivers = \"./x\"\n", []string{"analyzers.drivers"}},
		{"not toml", "this is = = not toml\n", []string{config.DefaultPath}},
		{"table spelt in another case", "[Tools]\nfmt = false\n", []string{"Tools", "tools"}},
		{"key spelt in another case", "[tools]\nFMT = false\n", []string{"tools.FMT", "tools.fmt"}},
		{"nested key spelt in another case", "[record.trust.high]\nMax_Review_Days = 3\n", []string{"max_review_days"}},
		{"negative max_total", "[record]\nmax_total = -1\nmax_per_rule = -2\nmax_policy = -3\n", []string{"record.max_total", "record.max_per_rule", "record.max_policy"}},
		{"negative max_review_days", "[record]\nmax_review_days = -5\n", []string{"record.max_review_days"}},
		{"negative tier limits", "[record.trust.low]\nmax_review_days = -1\nmax_count = -1\n", []string{"record.trust.low.max_review_days", "record.trust.low.max_count"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, dir, config.DefaultPath, tc.content)
			_, err := config.Load(dir, "")
			if err == nil {
				t.Fatal("Load accepted the file")
			}
			for _, sub := range tc.wantSub {
				if !strings.Contains(err.Error(), sub) {
					t.Errorf("error %q does not mention %q", err, sub)
				}
			}
		})
	}
}

func TestAnalyzersDriverIsAPackageInsideTheRoot(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, config.DefaultPath, "[analyzers]\ndriver = \"./internal/qa/vetdriver\"\n")
	got, err := config.Load(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	if got.Analyzers.Driver != "./internal/qa/vetdriver" || config.Default().Analyzers.Driver != "" {
		t.Errorf("driver = %q, default %q", got.Analyzers.Driver, config.Default().Analyzers.Driver)
	}
}

func TestModulesWithGoMod(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "nested/go.mod", "module example.com/nested\n")
	writeFile(t, dir, config.DefaultPath, "modules = [\"nested\"]\n")
	got, err := config.Load(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Modules, []string{"nested"}) {
		t.Errorf("modules = %v", got.Modules)
	}
}

func TestExplicitConfigPath(t *testing.T) {
	dir := t.TempDir()
	if _, err := config.Load(dir, "missing.toml"); err == nil {
		t.Error("a missing explicit config file must be an error")
	}
	writeFile(t, dir, "custom.toml", "[tools]\ndeadcode = false\n")
	got, err := config.Load(dir, "custom.toml")
	if err != nil {
		t.Fatal(err)
	}
	if got.Tools.Deadcode {
		t.Error("explicit file was not read")
	}
}

func TestAbsoluteConfigPathInsideTheRootIsRead(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "qa/other.toml", "[tools]\ndeadcode = false\n")
	got, err := config.Load(dir, filepath.Join(dir, "qa", "other.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if got.Tools.Deadcode {
		t.Error("the absolute path was not read")
	}
	outside := t.TempDir()
	writeFile(t, outside, "x.toml", "")
	_, err = config.Load(dir, filepath.Join(outside, "x.toml"))
	if err == nil || !strings.Contains(err.Error(), "outside the root") {
		t.Errorf("a config outside the root must say so, got %v", err)
	}
}

func TestProblemMessagesComeInAStableOrder(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, config.DefaultPath, "docs_dir = \"/a\"\nrecord = \"/b\"\n[summary]\nfile = \"/c\"\n[record.trust]\nmodels = { z = \"human\", y = \"human\", x = \"human\", w = \"human\" }\n")
	var first string
	for i := range 20 {
		_, err := config.Load(dir, "")
		if err == nil {
			t.Fatal("accepted")
		}
		if i == 0 {
			first = err.Error()
		} else if err.Error() != first {
			t.Fatalf("message order changed:\n%s\n%s", first, err)
		}
	}
}

const shellStage = "[[stage]]\nname = \"shellcheck\"\nphase = \"detectors\"\nsays = \"ShellCheck over every shell script\"\n" +
	"fix = \"Quote expansions\"\ncommand = [\"shellcheck\", \"{files}\"]\nfiles = [\"scripts/*.sh\"]\n"

func TestStagesAndResolversAreLoaded(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, config.DefaultPath, shellStage+
		"[[stage]]\nname = \"web\"\nphase = \"runners\"\nsays = \"s\"\nfix = \"f\"\ncommand = [\"web-test\"]\noutput = \"findings\"\ndir = \"web\"\n"+
		"[[resolver]]\nmatches = \"SC[0-9]*\"\ntext = \"See the ShellCheck wiki for {id}.\"\n")
	got, err := config.Load(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Stages) != 2 || got.Stages[0].Name != "shellcheck" || got.Stages[1].Dir != "web" || got.Stages[1].Output != "findings" {
		t.Errorf("stages = %+v", got.Stages)
	}
	if len(got.Resolvers) != 1 || got.Resolvers[0].Matches != "SC[0-9]*" {
		t.Errorf("resolvers = %+v", got.Resolvers)
	}
}

func TestLoadRejectsBadStagesAndResolvers(t *testing.T) {
	tests := map[string]struct {
		content string
		wantSub []string
	}{
		"unknown stage key":     {shellStage + "colour = \"red\"\n", []string{"stage.colour"}},
		"unknown resolver key":  {"[[resolver]]\nmatches = \"a\"\ntext = \"t\"\nextra = 1\n", []string{"resolver.extra"}},
		"stage without phase":   {"[[stage]]\nname = \"x\"\nsays = \"s\"\nfix = \"f\"\ncommand = [\"x\"]\n", []string{"stage `x`", "unknown phase"}},
		"stage declared twice":  {shellStage + shellStage, []string{"stage `shellcheck` is declared twice"}},
		"stage without files":   {"[[stage]]\nname = \"x\"\nphase = \"runners\"\nsays = \"s\"\nfix = \"f\"\ncommand = [\"x\", \"{files}\"]\n", []string{"selects no files"}},
		"stage escaping dir":    {shellStage + "dir = \"../up\"\n", []string{"dir \"../up\""}},
		"resolver bad glob":     {"[[resolver]]\nmatches = \"[\"\ntext = \"t\"\n", []string{"resolver `[`", "invalid glob"}},
		"resolver without text": {"[[resolver]]\nmatches = \"a*\"\ntext = \" \"\n", []string{"resolver `a*` has no text"}},
		"resolver no matches":   {"[[resolver]]\ntext = \"t\"\n", []string{"resolver has no `matches`"}},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, dir, config.DefaultPath, tc.content)
			_, err := config.Load(dir, "")
			if err == nil {
				t.Fatal("Load accepted the file")
			}
			for _, sub := range tc.wantSub {
				if !strings.Contains(err.Error(), sub) {
					t.Errorf("error %q does not mention %q", err, sub)
				}
			}
		})
	}
}
