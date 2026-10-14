package suppression

import (
	"fmt"
	"slices"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
	"go.yaml.in/yaml/v3"
)

const golangciRulePrefix = "golangci.yml::"

// golangciNames are the file names golangci-lint discovers its configuration under.
var golangciNames = []string{".golangci.yml", ".golangci.yaml", ".golangci.toml", ".golangci.json"}

func isGolangciConfig(base string) bool { return slices.Contains(golangciNames, base) }

// config is a parsed golangci-lint configuration. Lines are real for YAML and JSON; a TOML file is
// converted first, so its entries have no usable line.
type config struct {
	root  *yaml.Node
	lines bool
}

func parseGolangciConfig(name string, data []byte) (config, error) {
	if strings.HasSuffix(name, ".toml") {
		var generic map[string]any
		if err := toml.Unmarshal(data, &generic); err != nil {
			return config{}, fmt.Errorf("%w", err)
		}
		converted, err := yaml.Marshal(generic)
		if err != nil {
			return config{}, fmt.Errorf("%w", err)
		}
		data = converted
		return decodeYAML(data, false)
	}
	return decodeYAML(data, true)
}

func decodeYAML(data []byte, lines bool) (config, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return config{}, fmt.Errorf("%w", err)
	}
	root := &doc
	if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
		root = doc.Content[0]
	}
	return config{root: root, lines: lines}, nil
}

// line is the node's line, or 0 when the configuration had none to give.
func (c config) line(n *yaml.Node) int {
	if !c.lines || n == nil {
		return 0
	}
	return n.Line
}

// at follows mapping keys from n; nil when any is absent.
func at(n *yaml.Node, keys ...string) *yaml.Node {
	for _, key := range keys {
		if n == nil || n.Kind != yaml.MappingNode {
			return nil
		}
		var next *yaml.Node
		for i := 0; i+1 < len(n.Content); i += 2 {
			if n.Content[i].Value == key {
				next = n.Content[i+1]
				break
			}
		}
		n = next
	}
	return n
}

// items are the elements of a sequence, or the node itself when it is a scalar.
func items(n *yaml.Node) []*yaml.Node {
	switch {
	case n == nil:
		return nil
	case n.Kind == yaml.SequenceNode:
		return n.Content
	default:
		return []*yaml.Node{n}
	}
}

// listRoute is a setting holding a list of things that are switched off or exempted; each item is
// one suppression.
type listRoute struct {
	keys []string
	name string
	// keep selects the items that silence something and maps them to the detail; nil keeps all.
	keep func(string) (string, bool)
}

func removals(item string) (string, bool) {
	rest, ok := strings.CutPrefix(item, "-")
	return rest, ok
}

var listRoutes = []listRoute{
	{keys: []string{"linters", "exclusions", "paths"}, name: "exclusions.paths"},
	{keys: []string{"linters", "exclusions", "presets"}, name: "exclusions.presets"},
	{keys: []string{"formatters", "exclusions", "paths"}, name: "formatters.exclusions.paths"},
	{keys: []string{"linters", "disable"}, name: "linters.disable"},
	{keys: []string{"linters", "settings", "errcheck", "exclude-functions"}, name: "errcheck.exclude-functions"},
	{keys: []string{"linters", "settings", "gosec", "excludes"}, name: "gosec.excludes"},
	{keys: []string{"linters", "settings", "govet", "disable"}, name: "govet.disable"},
	{keys: []string{"linters", "settings", "gocritic", "disabled-checks"}, name: "gocritic.disabled-checks"},
	{keys: []string{"linters", "settings", "gocritic", "disabled-tags"}, name: "gocritic.disabled-tags"},
	{keys: []string{"linters", "settings", "staticcheck", "checks"}, name: "staticcheck.checks", keep: removals},
	{keys: []string{"linters", "settings", "testifylint", "disable"}, name: "testifylint.disable"},
}

// scalarRoute is a single setting that silences something when its value is not the strict one.
type scalarRoute struct {
	keys []string
	name string
	// fires reports whether the value silences something.
	fires func(string) bool
	// detail puts the value in the rule identifier, so that each value is its own decision.
	detail bool
}

func is(value string) func(string) bool { return func(got string) bool { return got == value } }

func isNot(values ...string) func(string) bool {
	return func(got string) bool { return !slices.Contains(values, got) }
}

func set(got string) bool { return got != "" && got != "false" }

var scalarRoutes = []scalarRoute{
	{keys: []string{"linters", "exclusions", "generated"}, name: "exclusions.generated", fires: isNot("disable"), detail: true},
	{keys: []string{"formatters", "exclusions", "generated"}, name: "formatters.exclusions.generated", fires: isNot("disable"), detail: true},
	{keys: []string{"linters", "default"}, name: "linters.default", fires: isNot("standard", "all"), detail: true},
	{keys: []string{"severity", "default"}, name: "severity.default", fires: isNot("error"), detail: true},
	{keys: []string{"run", "tests"}, name: "run.tests", fires: is("false")},
	{keys: []string{"issues", "new"}, name: "issues.new", fires: is("true")},
	{keys: []string{"issues", "new-from-rev"}, name: "issues.new-from-rev", fires: set, detail: true},
	{keys: []string{"issues", "new-from-merge-base"}, name: "issues.new-from-merge-base", fires: set, detail: true},
	{keys: []string{"issues", "new-from-patch"}, name: "issues.new-from-patch", fires: set, detail: true},
	{keys: []string{"linters", "settings", "govet", "disable-all"}, name: "govet.disable-all", fires: is("true")},
	{keys: []string{"linters", "settings", "testifylint", "disable-all"}, name: "testifylint.disable-all", fires: is("true")},
	{keys: []string{"linters", "settings", "revive", "ignore-generated-header"}, name: "revive.ignore-generated-header", fires: is("true")},
}

// issueCaps are the settings that, set above zero, hide the findings past a count. Left unset they
// are no route: go-qa's own run passes both as 0 on the command line, which overrides the file.
var issueCaps = []string{"max-issues-per-linter", "max-same-issues"}

// scanGolangci reads one golangci-lint configuration for settings that exclude findings or switch
// defences off.
func scanGolangci(name string, data []byte) fileScan {
	cfg, err := parseGolangciConfig(name, data)
	if err != nil {
		return unparseable(err)
	}
	found := cfg.lists(name)
	found = append(found, cfg.scalars(name)...)
	found = append(found, cfg.caps(name)...)
	found = append(found, cfg.exclusionRules(name)...)
	found = append(found, cfg.severityRules(name)...)
	found = append(found, cfg.reviveRules(name)...)
	return fileScan{found: found}
}

func (c config) suppression(name, rule string, line int) Suppression {
	return Suppression{Rule: golangciRulePrefix + rule, Path: name, Line: line, Route: RouteGolangciConfig}
}

func (c config) lists(name string) []Suppression {
	var found []Suppression
	for _, route := range listRoutes {
		for _, item := range items(at(c.root, route.keys...)) {
			value := item.Value
			if route.keep != nil {
				var ok bool
				if value, ok = route.keep(value); !ok {
					continue
				}
			}
			found = append(found, c.suppression(name, route.name+"::"+value, c.line(item)))
		}
	}
	return found
}

func (c config) scalars(name string) []Suppression {
	var found []Suppression
	for _, route := range scalarRoutes {
		node := at(c.root, route.keys...)
		if node == nil || node.Kind != yaml.ScalarNode || !route.fires(node.Value) {
			continue
		}
		rule := route.name
		if route.detail {
			rule += "::" + node.Value
		}
		found = append(found, c.suppression(name, rule, c.line(node)))
	}
	return found
}

func (c config) caps(name string) []Suppression {
	var found []Suppression
	for _, key := range issueCaps {
		if node := at(c.root, "issues", key); node != nil && node.Value != "0" {
			found = append(found, c.suppression(name, "issues."+key+"::"+node.Value, c.line(node)))
		}
	}
	return found
}

// linterDetail names the linters an entry applies to: them joined by plus, or * for all.
func linterDetail(entry *yaml.Node) string {
	var names []string
	for _, linter := range items(at(entry, "linters")) {
		names = append(names, linter.Value)
	}
	if len(names) == 0 {
		return "*"
	}
	return strings.Join(names, "+")
}

func (c config) exclusionRules(name string) []Suppression {
	var found []Suppression
	for _, entry := range items(at(c.root, "linters", "exclusions", "rules")) {
		detail := linterDetail(entry)
		if text := at(entry, "text"); text != nil && text.Value != "" {
			detail += "/" + text.Value
		}
		found = append(found, c.suppression(name, "exclusions.rules::"+detail, c.line(entry)))
	}
	return found
}

func (c config) severityRules(name string) []Suppression {
	var found []Suppression
	for _, entry := range items(at(c.root, "severity", "rules")) {
		severity := at(entry, "severity")
		if severity == nil || severity.Value == "" || severity.Value == "error" {
			continue
		}
		found = append(found, c.suppression(name, "severity.rules::"+linterDetail(entry)+"/"+severity.Value, c.line(entry)))
	}
	return found
}

func (c config) reviveRules(name string) []Suppression {
	var found []Suppression
	for _, entry := range items(at(c.root, "linters", "settings", "revive", "rules")) {
		rule := at(entry, "name")
		if rule == nil {
			continue
		}
		if disabled := at(entry, "disabled"); disabled != nil && disabled.Value == "true" {
			found = append(found, c.suppression(name, "revive.rules::"+rule.Value+"/disabled", c.line(disabled)))
		}
		if exclude := at(entry, "exclude"); exclude != nil && len(items(exclude)) > 0 {
			found = append(found, c.suppression(name, "revive.rules::"+rule.Value+"/exclude", c.line(exclude)))
		}
	}
	return found
}
