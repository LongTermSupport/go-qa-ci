package suppression_test

import "testing"

func TestStaticcheckConfRoutes(t *testing.T) {
	t.Parallel()
	const conf = "staticcheck.conf"
	cases := []struct {
		name string
		text string
		want []string
	}{
		// A check removed from the full set is switched off for all code: no record entry needed.
		{"checks removed from the full set", "checks = [\"all\", \"-SA1019\", \"-ST1000\"]\n", nil},
		{"checks removed from the inherited set", "checks = [\"inherit\", \"-SA1019\"]\n", nil},
		{"a narrowed list that also removes a check", "checks = [\"SA1019\", \"-SA1000\"]\n", []string{"staticcheck.conf::checks@" + conf + ":1"}},
		{"a check list that drops the rest", "checks = [\"SA1019\"]\n", []string{"staticcheck.conf::checks@" + conf + ":1"}},
		{"inherited list", "checks = [\"inherit\", \"SA1019\"]\n", nil},
		{"whitelists", "checks = [\"all\"]\ninitialisms = [\"ID\"]\ndot_import_whitelist = [\"x\"]\nhttp_status_code_whitelist = [\"200\"]\n", []string{
			"staticcheck.conf::initialisms@" + conf + ":2", "staticcheck.conf::dot_import_whitelist@" + conf + ":3", "staticcheck.conf::http_status_code_whitelist@" + conf + ":4",
		}},
		{"nothing silenced", "checks = [\"all\"]\n", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			wantExactly(t, ruleLines(scan(t, map[string]string{conf: tc.text})), tc.want...)
		})
	}
}

func TestStaticcheckConfInASubdirectoryAndUnparseable(t *testing.T) {
	t.Parallel()
	result := scan(t, map[string]string{"pkg/staticcheck.conf": "checks = [\"SA1019\"]\n", "other/staticcheck.conf": "checks = ["})
	wantExactly(t, ruleLines(result), "staticcheck.conf::checks@pkg/staticcheck.conf:1")
	if len(result.Unparseable) != 1 || result.Unparseable[0].Path != "other/staticcheck.conf" {
		t.Errorf("got %+v", result.Unparseable)
	}
}
