package record

import (
	"fmt"
	"strings"
)

const ratchetTable = "[ratchet]"

// SetHighWater rewrites the high_water value inside the [ratchet] table, keeping layout and
// comments. It reports false when the text has no such line.
func SetHighWater(text string, value int) (string, bool) {
	table := ""
	var out strings.Builder
	done := false
	for _, line := range strings.SplitAfter(text, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "["):
			table = trimmed
		case !done && table == ratchetTable && isHighWaterLine(trimmed):
			key, rest, _ := strings.Cut(line, "=")
			valueText := strings.TrimLeft(rest, " \t")
			spacing := rest[:len(rest)-len(valueText)]
			tail := strings.TrimLeft(valueText, "0123456789_")
			line = fmt.Sprintf("%s=%s%d%s", key, spacing, value, tail)
			done = true
		}
		out.WriteString(line)
	}
	return out.String(), done
}

func isHighWaterLine(trimmed string) bool {
	rest, ok := strings.CutPrefix(trimmed, "high_water")
	return ok && strings.HasPrefix(strings.TrimLeft(rest, " \t"), "=")
}

// RemoveExceptions drops the exceptions at indexes (0-based, file order), each with the comment
// lines directly above its [[exception]] header. An index past the last entry removes nothing.
func RemoveExceptions(text string, indexes []int) string {
	lines := strings.SplitAfter(text, "\n")
	isComment := func(i int) bool { return strings.HasPrefix(strings.TrimSpace(lines[i]), "#") }
	var tables, entries []int
	for _, h := range headers([]byte(text)) {
		tables = append(tables, h.line-1)
		if h.exception {
			entries = append(entries, h.line-1)
		}
	}
	drop := make([]bool, len(lines))
	for _, index := range indexes {
		if index < 0 || index >= len(entries) {
			continue
		}
		header := entries[index]
		start := header
		for start > 0 && isComment(start-1) {
			start--
		}
		end := len(lines)
		for _, h := range tables {
			if h > header {
				end = h
				break
			}
		}
		for end > header+1 && isComment(end-1) {
			end--
		}
		for i := start; i < end; i++ {
			drop[i] = true
		}
	}
	var out strings.Builder
	for i, line := range lines {
		if !drop[i] {
			out.WriteString(line)
		}
	}
	return out.String()
}
