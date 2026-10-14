package runners_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/faketool"
	"github.com/LongTermSupport/go-qa-ci/internal/finding"
	"github.com/LongTermSupport/go-qa-ci/internal/runners"
)

// stream has one reached vulnerability (symbol level), one only present in a module, one only
// imported (package level) and a duplicate of the reached one.
const stream = `{"config":{"scanner_name":"govulncheck","scan_level":"symbol"}}
{"progress":{"message":"Fetching vulnerabilities from the database..."}}
{"osv":{"id":"GO-2000-0001","summary":"Panic in parsing"}}
{"osv":{"id":"GO-2000-0002","summary":"Unreached"}}
{"finding":{"osv":"GO-2000-0002","fixed_version":"v1.2.0","trace":[{"module":"example.org/dep","version":"v1.0.0"}]}}
{"finding":{"osv":"GO-2000-0002","fixed_version":"v1.2.0","trace":[{"module":"example.org/dep","version":"v1.0.0","package":"example.org/dep/parse"}]}}
{"finding":{"osv":"GO-2000-0001","fixed_version":"v1.1.0","trace":[{"module":"example.org/dep","version":"v1.0.0","package":"example.org/dep/parse","function":"Parse","position":{"filename":"parse/parse.go","line":12}},{"module":"example.com/fixture","package":"example.com/fixture","function":"main","position":{"filename":"main.go","line":5}}]}}
{"finding":{"osv":"GO-2000-0001","fixed_version":"v1.1.0","trace":[{"module":"example.org/dep","version":"v1.0.0","package":"example.org/dep/parse","function":"Parse","position":{"filename":"parse/parse.go","line":12}},{"module":"example.com/fixture","package":"example.com/fixture","function":"other","position":{"filename":"other.go","line":9}}]}}
`

func TestParseVulnReportsOnlyReachedVulnerabilitiesOncePerModule(t *testing.T) {
	got, err := runners.ParseVuln(strings.NewReader(stream))
	if err != nil {
		t.Fatal(err)
	}
	want := finding.Finding{
		Rule: "GO-2000-0001", Path: "main.go", Line: 5,
		Message: "Panic in parsing: example.org/dep/parse example.org/dep@v1.0.0 is called through Parse; fixed in v1.1.0",
	}
	if len(got) != 1 || got[0] != want {
		t.Errorf("got %+v, want [%+v]", got, want)
	}
}

func TestParseVulnRejectsAMalformedStream(t *testing.T) {
	if _, err := runners.ParseVuln(strings.NewReader(`{"osv":`)); err == nil {
		t.Error("a truncated stream was accepted")
	}
}

func vulnCall(t *testing.T) error {
	t.Helper()
	if _, err := runners.Vuln(ctx(), t.TempDir()); err != nil {
		return fmt.Errorf("vuln: %w", err)
	}
	return nil
}

func TestVulnRunsGovulncheckAsJSONOverEveryPackage(t *testing.T) {
	log := faketool.Install(t, "govulncheck", faketool.Spec{Stdout: stream})
	dir := t.TempDir()
	got, err := runners.Vuln(ctx(), dir)
	if err != nil || len(got) != 1 {
		t.Fatalf("got %v, %v", got, err)
	}
	calls := faketool.Calls(t, log)
	if len(calls) != 1 || !strings.HasSuffix(calls[0], "\t-format json ./...") {
		t.Errorf("calls: %q", calls)
	}
}

func TestVulnFailsWhenTheScanCannotRun(t *testing.T) {
	faketool.Install(t, "govulncheck", faketool.Spec{Stderr: "database unreachable\n", Code: 2})
	if err := vulnCall(t); err == nil || !strings.Contains(err.Error(), "database unreachable") {
		t.Errorf("got %v", err)
	}
}
