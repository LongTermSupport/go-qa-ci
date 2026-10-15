// Package toolstore installs the external tools tools.lock pins into a directory go-qa manages,
// keyed by version, and makes go-qa resolve those copies before anything else on PATH.
//
// An install is verified before it is placed (a release archive against the sha256 the lock pins,
// a Go tool by the Go checksum database), built in a scratch directory and renamed into place, so
// a failure never leaves a tool that looks installed.
package toolstore

import (
	"bufio"
	"fmt"
	"slices"
	"strings"
)

// Kind is how a tool is obtained.
type Kind int

// The two ways tools.lock pins a tool.
const (
	// KindGo is built from source with `go install <package>@<version>`.
	KindGo Kind = iota
	// KindRelease is the publisher's linux/amd64 release archive, verified against SHA256.
	KindRelease
)

// Tool is one pinned tool.
type Tool struct {
	Name    string
	Kind    Kind
	Version string
	// Package is the Go package of a KindGo tool.
	Package string
	// SHA256 is the pinned checksum of a KindRelease tool's archive; empty when the lock has none.
	SHA256 string
}

// Lock is the parsed tools.lock.
type Lock struct {
	tools map[string]Tool
}

// ParseLock reads the text of tools.lock.
func ParseLock(text string) (Lock, error) {
	lock := Lock{tools: map[string]Tool{}}
	sums := map[string]string{}
	section := ""
	scanner := bufio.NewScanner(strings.NewReader(text))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			section = strings.Trim(line, "[]")
			if !slices.Contains([]string{"go", "release", "checksums"}, section) {
				return Lock{}, fmt.Errorf("tools.lock: unknown section [%s]", section)
			}
			continue
		}
		name, raw, ok := strings.Cut(line, " = ")
		value := strings.Trim(raw, `"`)
		if !ok || section == "" || name == "" || value == "" {
			return Lock{}, fmt.Errorf("tools.lock: malformed line %q", line)
		}
		switch section {
		case "go":
			pkg, version, found := strings.Cut(value, "@")
			if !found {
				return Lock{}, fmt.Errorf("tools.lock: %s = %q is not package@version", name, value)
			}
			lock.tools[name] = Tool{Name: name, Kind: KindGo, Version: version, Package: pkg}
		case "release":
			lock.tools[name] = Tool{Name: name, Kind: KindRelease, Version: value}
		default:
			sums[name] = value
		}
	}
	if err := scanner.Err(); err != nil {
		return Lock{}, fmt.Errorf("reading tools.lock: %w", err)
	}
	for name, sum := range sums {
		tool, found := lock.tools[name]
		if !found || tool.Kind != KindRelease {
			return Lock{}, fmt.Errorf("tools.lock: [checksums] pins %s, which is not in [release]", name)
		}
		tool.SHA256 = sum
		lock.tools[name] = tool
	}
	return lock, nil
}

// Names are the pinned tools, sorted.
func (l Lock) Names() []string {
	names := make([]string, 0, len(l.tools))
	for name := range l.tools {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// Tool is the pin for name; a name tools.lock does not pin is an error.
func (l Lock) Tool(name string) (Tool, error) {
	tool, ok := l.tools[name]
	if !ok {
		return Tool{}, fmt.Errorf("%s is not pinned in tools.lock", name)
	}
	return tool, nil
}
