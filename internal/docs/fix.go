package docs

import "strings"

// Fix is the first paragraph, on one line, of the first section of a page whose heading is one of
// headings (compared without case, at any heading level); empty when no such section has text.
// The agent summary quotes it so the correct construction is read before a failure.
func Fix(page string, headings []string) string {
	lines := strings.Split(page, "\n")
	for _, want := range headings {
		for i, line := range lines {
			if heading, isHeading := headingText(line); isHeading && strings.EqualFold(heading, strings.TrimSpace(want)) {
				if paragraph := paragraphAfter(lines[i+1:]); paragraph != "" {
					return paragraph
				}
			}
		}
	}
	return ""
}

func headingText(line string) (string, bool) {
	if !strings.HasPrefix(line, "#") {
		return "", false
	}
	return strings.TrimSpace(strings.TrimLeft(line, "#")), true
}

// paragraphAfter joins the first run of non-blank lines, ending early at a heading.
func paragraphAfter(lines []string) string {
	var words []string
	for _, line := range lines {
		if _, isHeading := headingText(line); isHeading {
			break
		}
		if strings.TrimSpace(line) == "" {
			if len(words) > 0 {
				break
			}
			continue
		}
		words = append(words, strings.Fields(line)...)
	}
	return strings.Join(words, " ")
}
