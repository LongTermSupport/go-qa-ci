package configs_test

import (
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/LongTermSupport/go-qa-ci/configs"
)

// wantLinters are the linters the baseline enables on top of golangci-lint's standard set: the ones
// major Go projects enable, the narrow bug patterns, and gocognit as the one complexity measure.
var wantLinters = []string{
	"bodyclose", "durationcheck", "errchkjson", "errorlint", "exhaustive", "gocognit", "gocritic",
	"gosec", "misspell", "nilerr", "nilnil", "nolintlint", "predeclared", "revive", "testifylint",
	"thelper", "unconvert", "unparam", "usestdlibvars",
}

// offLinters are more noise than signal, or enforce a style Go itself does not ask for.
var offLinters = []string{
	"wrapcheck", "depguard", "forcetypeassert", "errname", "noctx", "contextcheck", "tparallel",
	"rowserrcheck", "sqlclosecheck", "dupword", "goconst", "exhaustruct", "varnamelen", "wsl",
	"ireturn", "err113", "gochecknoglobals", "paralleltest", "testpackage", "lll", "mnd", "godot",
	"cyclop", "gocyclo", "funlen", "nestif",
}

// wantGovet are the analyzers gopls runs by default beyond vet's own set.
var wantGovet = []string{"atomicalign", "deepequalerrors", "nilness", "sortslice", "timeformat", "unusedwrite"}

func baseline(t *testing.T) *yaml.Node {
	t.Helper()
	var doc yaml.Node
	if err := yaml.Unmarshal(configs.Golangci(), &doc); err != nil {
		t.Fatal(err)
	}
	return doc.Content[0]
}

func at(n *yaml.Node, keys ...string) *yaml.Node {
	for _, key := range keys {
		if n == nil || n.Kind != yaml.MappingNode {
			return nil
		}
		var next *yaml.Node
		for i := 0; i+1 < len(n.Content); i += 2 {
			if n.Content[i].Value == key {
				next = n.Content[i+1]
			}
		}
		n = next
	}
	return n
}

func values(t *testing.T, n *yaml.Node) []string {
	t.Helper()
	if n == nil || n.Kind != yaml.SequenceNode {
		t.Fatalf("not a list: %v", n)
	}
	out := make([]string, 0, len(n.Content))
	for _, item := range n.Content {
		if strings.TrimSpace(item.LineComment) == "" {
			t.Errorf("%s has no comment saying why it is there", item.Value)
		}
		out = append(out, item.Value)
	}
	return out
}

// enableAll finds every enable-all key, at any depth.
func enableAll(n *yaml.Node, path string) []string {
	var found []string
	if n.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(n.Content); i += 2 {
			key := path + "." + n.Content[i].Value
			if n.Content[i].Value == "enable-all" {
				found = append(found, key)
			}
			found = append(found, enableAll(n.Content[i+1], key)...)
		}
	}
	for _, child := range n.Content {
		if n.Kind != yaml.MappingNode {
			found = append(found, enableAll(child, path)...)
		}
	}
	return found
}

func TestBaselineIsStandardPlusTheStatedExtras(t *testing.T) {
	t.Parallel()
	root := baseline(t)
	if got := at(root, "linters", "default"); got == nil || got.Value != "standard" {
		t.Errorf("linters.default = %v, want standard", got)
	}
	got := values(t, at(root, "linters", "enable"))
	slices.Sort(got)
	if !slices.Equal(got, wantLinters) {
		t.Errorf("enabled linters %v, want exactly %v", got, wantLinters)
	}
	for _, off := range offLinters {
		if slices.Contains(got, off) {
			t.Errorf("%s is enabled; it is off in the baseline", off)
		}
	}
	if found := enableAll(root, ""); len(found) != 0 {
		t.Errorf("enable-all is set at %v", found)
	}
}

func TestBaselineGovetIsVetDefaultsPlusGoplsDefaults(t *testing.T) {
	t.Parallel()
	govet := at(baseline(t), "linters", "settings", "govet")
	got := values(t, at(govet, "enable"))
	slices.Sort(got)
	if !slices.Equal(got, wantGovet) {
		t.Errorf("govet enables %v, want exactly %v", got, wantGovet)
	}
	for _, key := range []string{"enable-all", "disable-all", "disable"} {
		if at(govet, key) != nil {
			t.Errorf("govet sets %s", key)
		}
	}
}

// gocritic and revive run with their default checks only.
func TestBaselineKeepsGocriticAndReviveAtTheirDefaults(t *testing.T) {
	t.Parallel()
	for _, linter := range []string{"gocritic", "revive"} {
		if at(baseline(t), "linters", "settings", linter) != nil {
			t.Errorf("%s has settings; the baseline runs its default checks only", linter)
		}
	}
}
