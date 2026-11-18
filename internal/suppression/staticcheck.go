package suppression

import (
	"slices"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
)

const (
	staticcheckConfRule = "staticcheck.conf::"
	// checksAll and checksInherit are the entries of a checks list that keep the default set.
	checksAll     = "all"
	checksInherit = "inherit"
)

// whitelistKeys are staticcheck.conf settings that soften checks without naming one to disable.
var whitelistKeys = []string{"initialisms", "dot_import_whitelist", "http_status_code_whitelist"}

// scanStaticcheckConf reads one staticcheck.conf: a checks list narrowed to named checks, and the
// whitelists that exempt names from checks. A check removed with `-XXXX` from the full or inherited
// set is switched off for all code, which is choosing the defence set and no suppression.
func scanStaticcheckConf(name string, data []byte) fileScan {
	var conf map[string]any
	if err := toml.Unmarshal(data, &conf); err != nil {
		return unparseable(err)
	}
	text := string(data)
	var found []Suppression
	if checks, ok := conf["checks"].([]any); ok {
		keepsDefaults := slices.ContainsFunc(checks, func(entry any) bool {
			check, isString := entry.(string)
			return isString && (check == checksAll || check == checksInherit)
		})
		if !keepsDefaults {
			line := keyLine(text, "checks")
			found = append(found, Suppression{Rule: staticcheckConfRule + "checks", Path: name, Line: line, Route: RouteStaticcheckConf})
		}
	}
	for _, key := range whitelistKeys {
		if _, ok := conf[key]; ok {
			found = append(found, Suppression{Rule: staticcheckConfRule + key, Path: name, Line: keyLine(text, key), Route: RouteStaticcheckConf})
		}
	}
	return fileScan{found: found}
}

// keyLine is the 1-based line on which a TOML key is assigned, or 0.
func keyLine(text, key string) int {
	for i, line := range strings.Split(text, "\n") {
		rest, ok := strings.CutPrefix(strings.TrimSpace(line), key)
		if ok && strings.HasPrefix(strings.TrimSpace(rest), "=") {
			return i + 1
		}
	}
	return 0
}
