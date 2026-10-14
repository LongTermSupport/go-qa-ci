package docs_test

import (
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/docs"
)

func TestFixQuotesTheFirstParagraphUnderTheNamedHeading(t *testing.T) {
	t.Parallel()
	text := "# id\n\n## Why\n\nBecause.\n\n## How to fix correctly\n\nWrite it\nthis way.\n\nSecond paragraph.\n\n## Suppression\n\nNo.\n"
	if got := docs.Fix(text, []string{"how to fix correctly"}); got != "Write it this way." {
		t.Errorf("got %q", got)
	}
}

func TestFixTriesTheHeadingsInOrderAndIsEmptyWhenNoneMatches(t *testing.T) {
	t.Parallel()
	text := "# id\n\n### Remediation\n\nDo this.\n"
	if got := docs.Fix(text, []string{"How to fix correctly", "Remediation"}); got != "Do this." {
		t.Errorf("got %q", got)
	}
	if got := docs.Fix(text, []string{"Nothing"}); got != "" {
		t.Errorf("got %q", got)
	}
}

func TestFixStopsAtTheNextHeading(t *testing.T) {
	t.Parallel()
	if got := docs.Fix("## Fix\n## Next\n\nText.\n", []string{"Fix"}); got != "" {
		t.Errorf("got %q", got)
	}
}
