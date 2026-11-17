package docs

import (
	"strings"
	"testing"
	"testing/fstest"
)

func auditFS(t *testing.T, pages map[string]string, ids []string) []string {
	t.Helper()
	fsys := fstest.MapFS{}
	for id, text := range pages {
		fsys["rules/"+id+".md"] = &fstest.MapFile{Data: []byte(text)}
	}
	var messages []string
	for _, f := range auditPages(fsys, ids) {
		messages = append(messages, f.Path+": "+f.Message)
	}
	return messages
}

func TestAuditPagesFlagsEachDefect(t *testing.T) {
	t.Parallel()
	good := "# a.rule\n\n## How to fix correctly\n\nDo this.\n"
	tests := []struct {
		name  string
		pages map[string]string
		want  string
		ids   []string
	}{
		{"clean", map[string]string{"a.rule": good}, "", []string{"a.rule"}},
		{"missing page", map[string]string{"b.rule": strings.ReplaceAll(good, "a.rule", "b.rule")}, "no bundled page", []string{"a.rule", "b.rule"}},
		{"orphan bundled page", map[string]string{"a.rule": good, "b.rule": good}, "documents no built-in", []string{"a.rule"}},
		{"wrong first line", map[string]string{"a.rule": "# other\n\n## How to fix correctly\n\nDo this.\n"}, "first line", []string{"a.rule"}},
		{"no fix section", map[string]string{"a.rule": "# a.rule\n\nText.\n"}, "How to fix correctly", []string{"a.rule"}},
		{"section only in prose", map[string]string{"a.rule": "# a.rule\n\nSee How to fix correctly.\n"}, "How to fix correctly", []string{"a.rule"}},
		{"empty section", map[string]string{"a.rule": "# a.rule\n\n## How to fix correctly\n\n## Next\n"}, "empty", []string{"a.rule"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := auditFS(t, tt.pages, tt.ids)
			if tt.want == "" {
				if len(got) != 0 {
					t.Fatalf("unexpected findings: %v", got)
				}
				return
			}
			if len(got) != 1 || !strings.Contains(got[0], tt.want) {
				t.Fatalf("findings = %v, want one containing %q", got, tt.want)
			}
		})
	}
}
