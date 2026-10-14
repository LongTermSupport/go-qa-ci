// Package selfgate proves TOOLING-SPEC 8.2 for the golangci rules: every linter, setting,
// formatter and issue cap in the bundled baseline is active when go-qa analyses itself, and the
// only differences are the documented configuration exceptions.
package selfgate_test

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/LongTermSupport/go-qa-ci/configs"
)

func readYAML(t *testing.T, path string) map[string]any {
	t.Helper()
	return decode(t, readRepoFile(t, path), path)
}

func decode(t *testing.T, data []byte, name string) map[string]any {
	t.Helper()
	var out map[string]any
	if err := yaml.Unmarshal(data, &out); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return out
}

// repoPath names a file relative to the module root, slash-separated.
func repoPath(elem ...string) string {
	return filepath.ToSlash(filepath.Join(elem...))
}

// readRepoFile reads a module-relative file through os.Root, which cannot be steered outside the
// module root.
func readRepoFile(t *testing.T, path string) []byte {
	t.Helper()
	root, err := os.OpenRoot(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := root.Close(); closeErr != nil {
			t.Error(closeErr)
		}
	}()
	data, err := root.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// missing reports every baseline value that want does not carry: map keys recurse, list items
// must be present, scalars must be equal.
func missing(base, got any, path string) []string {
	switch b := base.(type) {
	case map[string]any:
		g, ok := got.(map[string]any)
		if !ok {
			return []string{path + ": not a mapping"}
		}
		var out []string
		for _, key := range slices.Sorted(maps.Keys(b)) {
			value := b[key]
			child, present := g[key]
			if !present {
				out = append(out, path+"."+key+": absent")
				continue
			}
			out = append(out, missing(value, child, path+"."+key)...)
		}
		return out
	case []any:
		g, ok := got.([]any)
		if !ok {
			return []string{path + ": not a list"}
		}
		var out []string
		for _, item := range b {
			if !slices.ContainsFunc(g, func(c any) bool { return reflect.DeepEqual(item, c) }) {
				out = append(out, fmt.Sprintf("%s: %v absent", path, item))
			}
		}
		return out
	default:
		if !reflect.DeepEqual(base, got) {
			return []string{fmt.Sprintf("%s: %v != %v", path, base, got)}
		}
		return nil
	}
}

// merge applies overlay on top of base: mappings merge, lists append, and a scalar the baseline
// already sets cannot be changed. That does not stop an overlay weakening the baseline through a key
// the baseline leaves unset (gosec.excludes, linters.exclusions.rules); TestOverlayHoldsOnlyDocumentedExceptions
// pins the overlay's exact content for that reason.
func merge(base, overlay any, path string) (any, error) {
	switch o := overlay.(type) {
	case map[string]any:
		b, ok := base.(map[string]any)
		if base != nil && !ok {
			return nil, fmt.Errorf("%s: overlay mapping over a non-mapping", path)
		}
		out := map[string]any{}
		for k, v := range b {
			out[k] = v
		}
		for k, v := range o {
			merged, err := merge(out[k], v, path+"."+k)
			if err != nil {
				return nil, err
			}
			out[k] = merged
		}
		return out, nil
	case []any:
		b, ok := base.([]any)
		if base != nil && !ok {
			return nil, fmt.Errorf("%s: overlay list over a non-list", path)
		}
		return append(slices.Clone(b), o...), nil
	default:
		if base != nil && !reflect.DeepEqual(base, overlay) {
			return nil, fmt.Errorf("%s: overlay overrides the baseline value %v", path, base)
		}
		return overlay, nil
	}
}

func TestBundledBaselineIsWhatTheEmbeddedCopyHolds(t *testing.T) {
	t.Parallel()
	onDisk := readRepoFile(t, repoPath("configs", "golangci.yml"))
	if string(onDisk) != string(configs.Golangci()) {
		t.Fatal("the embedded baseline differs from configs/golangci.yml")
	}
}

// The self-gate runs the root .golangci.yml, so that file must be exactly baseline + overlay.
func TestSelfGateConfigIsBaselinePlusOverlay(t *testing.T) {
	t.Parallel()
	baseline := decode(t, configs.Golangci(), "baseline")
	overlay := readYAML(t, repoPath("qa", "golangci-overlay.yml"))
	own := readYAML(t, repoPath(".golangci.yml"))

	want, err := merge(baseline, overlay, "")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, any(own)) {
		t.Fatalf(".golangci.yml is not configs/golangci.yml plus qa/golangci-overlay.yml:\nwant %v\n got %v", want, own)
	}
}

// Independently of how the file was derived: every linter, setting, formatter and cap the
// baseline carries is present in the configuration the self-gate runs.
func TestEveryBaselineDefenceIsActiveOnTheToolchainItself(t *testing.T) {
	t.Parallel()
	baseline := decode(t, configs.Golangci(), "baseline")
	own := readYAML(t, repoPath(".golangci.yml"))
	if gaps := missing(baseline, own, ""); len(gaps) != 0 {
		t.Fatalf("the self-gate configuration drops baseline defences (TOOLING-SPEC 8.2):\n%s", strings.Join(gaps, "\n"))
	}
}

func TestCheckerDetectsADroppedLinterSettingAndCap(t *testing.T) {
	t.Parallel()
	baseline := decode(t, configs.Golangci(), "baseline")
	drops := map[string]string{
		"linter":    "linters:\n  default: standard\n  enable: [gosec]\n",
		"formatter": "formatters:\n  enable: [gofumpt]\n",
		"setting":   "linters:\n  settings:\n    errcheck:\n      check-blank: false\n",
		"cap":       "issues:\n  max-issues-per-linter: 50\n",
	}
	for name, content := range drops {
		got := decode(t, []byte(content), name)
		if len(missing(baseline, got, "")) == 0 {
			t.Errorf("a configuration with a dropped %s was accepted", name)
		}
	}
}

func TestOverlayCannotOverrideABaselineValue(t *testing.T) {
	t.Parallel()
	baseline := decode(t, configs.Golangci(), "baseline")
	overlay := decode(t, []byte("issues:\n  max-issues-per-linter: 50\n"), "overlay")
	_, err := merge(baseline, overlay, "")
	if err == nil {
		t.Fatal("an overlay overriding a baseline scalar was accepted")
	}
}

func asList(t *testing.T, v any) []any {
	t.Helper()
	list, ok := v.([]any)
	if !ok {
		t.Fatalf("not a list: %v", v)
	}
	return list
}

func asMap(t *testing.T, v any) map[string]any {
	t.Helper()
	m, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("not a mapping: %v", v)
	}
	return m
}

// The overlay holds only the documented configuration exceptions plus the project's dependency
// allow-list; each exception is named in docs/record.md for the suppression detector (M6).
func TestOverlayHoldsOnlyDocumentedExceptions(t *testing.T) {
	t.Parallel()
	overlay := readYAML(t, repoPath("qa", "golangci-overlay.yml"))
	if keys := mapKeys(overlay); !slices.Equal(keys, []string{"linters"}) {
		t.Fatalf("overlay top-level keys %v, want only linters", keys)
	}
	linters := asMap(t, overlay["linters"])
	if keys := mapKeys(linters); !slices.Equal(keys, []string{"enable", "exclusions", "settings"}) {
		t.Fatalf("overlay linters keys %v", keys)
	}
	if got := asList(t, linters["enable"]); !reflect.DeepEqual(got, []any{"depguard"}) {
		t.Fatalf("overlay enables %v, want only depguard", got)
	}
	settings := asMap(t, linters["settings"])
	if keys := mapKeys(settings); !slices.Equal(keys, []string{"depguard", "gosec"}) {
		t.Fatalf("overlay settings %v, want depguard and gosec only", keys)
	}
	gosec := asMap(t, settings["gosec"])
	if keys := mapKeys(gosec); !slices.Equal(keys, []string{"excludes"}) {
		t.Fatalf("overlay gosec keys %v", keys)
	}
	if got := asList(t, gosec["excludes"]); !reflect.DeepEqual(got, []any{"G204"}) {
		t.Fatalf("overlay gosec excludes %v, want only G204", got)
	}
	exclusions := asMap(t, linters["exclusions"])
	rules := asList(t, exclusions["rules"])
	if len(rules) != 1 {
		t.Fatalf("overlay exclusion rules %v, want only the G306 test file rule", rules)
	}
	rule := asMap(t, rules[0])
	if rule["text"] != "G306" || rule["path"] != `tests/scripts/secret_scan_test\.go` {
		t.Fatalf("unexpected exclusion rule %v", rule)
	}

	docs := readRepoFile(t, repoPath("docs", "record.md"))
	for _, exception := range []string{"G204", "G306"} {
		if !strings.Contains(string(docs), "gosec `"+exception+"`") {
			t.Errorf("docs/record.md does not list the %s configuration exception", exception)
		}
	}
}

func mapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

func TestRepositoryRootIsFound(t *testing.T) {
	t.Parallel()
	if len(readRepoFile(t, repoPath("go.mod"))) == 0 {
		t.Fatal("tests/selfgate is not two levels below the module root")
	}
}
