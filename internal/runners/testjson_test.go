package runners_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/runners"
)

const sampleStream = `{"Action":"output","Package":"p/a","Test":"TestSlow","Output":"=== RUN   TestSlow\n"}
{"Action":"pass","Package":"p/a","Test":"TestSlow","Elapsed":2.5}
{"Action":"pass","Package":"p/a","Test":"TestFast","Elapsed":0.01}
{"Action":"output","Package":"p/a","Output":"ok  \tp/a\t2.6s\n"}
{"Action":"pass","Package":"p/a","Elapsed":2.6}
{"Action":"fail","Package":"p/b","Test":"TestBroken","Elapsed":1.25}
{"Action":"fail","Package":"p/b","Elapsed":1.3}
{"Action":"skip","Package":"p/c","Test":"TestSkipped","Elapsed":9}
not json at all
`

func textOf(stream string) string {
	parsed := runners.ParseTestJSON(stream)
	return parsed.Text
}

func timingsOf(stream string) runners.Timings {
	return runners.ParseTestJSON(stream).Timings
}

func TestParseTestJSONRebuildsTheTextOfTheRun(t *testing.T) {
	text := textOf(sampleStream)
	for _, want := range []string{"=== RUN   TestSlow\n", "ok  \tp/a\t2.6s\n", "not json at all"} {
		if !strings.Contains(text, want) {
			t.Errorf("text lost %q:\n%s", want, text)
		}
	}
}

func TestSlowestListsPackagesAndTestsLongestFirst(t *testing.T) {
	report := timingsOf(sampleStream).Slowest(10)
	for _, want := range []string{"slowest packages", "slowest tests", "p/a", "2.60s", "p/b.TestBroken", "1.25s", "p/a.TestSlow"} {
		if !strings.Contains(report, want) {
			t.Errorf("report lacks %q:\n%s", want, report)
		}
	}
	if strings.Contains(report, "TestSkipped") {
		t.Errorf("a skipped test is not timed:\n%s", report)
	}
	if strings.Index(report, "p/a.TestSlow") > strings.Index(report, "p/a.TestFast") {
		t.Errorf("not longest first:\n%s", report)
	}
}

func TestSlowestKeepsOnlyTheLimit(t *testing.T) {
	report := timingsOf(sampleStream).Slowest(1)
	if strings.Contains(report, "TestFast") || strings.Contains(report, "p/b\n") {
		t.Errorf("report is longer than 1 line per list:\n%s", report)
	}
}

func TestSlowestIsEmptyWithoutTimings(t *testing.T) {
	if report := timingsOf("").Slowest(10); report != "" {
		t.Errorf("got %q", report)
	}
}

func TestTestReportsTheSlowestPackagesAndTests(t *testing.T) {
	dir := module(t, map[string]string{
		"f.go":      passingSource,
		"f_test.go": "package fixture\n\nimport \"testing\"\n\nfunc TestDouble(t *testing.T) {\n\tif Double(2) != 4 {\n\t\tt.Fatal(\"bad\")\n\t}\n}\n",
	})
	var report bytes.Buffer
	output, passed, err := runners.Test(ctx(), dir, runners.TestPlan{Report: &report})
	if err != nil || !passed {
		t.Fatalf("got %q, %v, %v", output, passed, err)
	}
	if !strings.Contains(output, "ok") || strings.Contains(output, `"Action"`) {
		t.Errorf("output is not the plain go test text:\n%s", output)
	}
	for _, want := range []string{"slowest packages", "example.com/fixture", "slowest tests", "TestDouble"} {
		if !strings.Contains(report.String(), want) {
			t.Errorf("report lacks %q:\n%s", want, report.String())
		}
	}
}
