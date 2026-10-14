package selfgate_test

import (
	"path/filepath"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/record"
	"github.com/LongTermSupport/go-qa-ci/internal/suppression"
)

func ownScan(t *testing.T) suppression.Result {
	t.Helper()
	cfg := loadOwnConfig(t)
	result, err := suppression.Scan(filepath.Join("..", ".."), suppression.Options{Exclude: cfg.ScanExclude})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func ownRecord(t *testing.T) record.Record {
	t.Helper()
	cfg := loadOwnConfig(t)
	rec, err := record.Load(filepath.Join("..", "..", filepath.FromSlash(cfg.Record.Path)))
	if err != nil {
		t.Fatal(err)
	}
	return rec
}

// The toolchain's own source and configuration silence nothing the project record does not name,
// and the record names nothing that is not used (TOOLING-SPEC 4.3 on the toolchain itself).
func TestOwnSuppressionsAreExactlyTheRecordedOnes(t *testing.T) {
	t.Parallel()
	matcher := ownRecord(t).Matcher()
	for _, f := range suppression.Check(ownScan(t), matcher) {
		t.Errorf("%s", f)
	}
	for _, f := range matcher.StaleCovering(record.CoversSuppressions, "record") {
		t.Errorf("%s", f)
	}
}

// Without its record the repository's configuration exceptions are findings: the check is not
// vacuous, and each exception the documentation lists is a route the detector sees.
func TestOwnConfigurationExceptionsAreSeenByTheDetector(t *testing.T) {
	t.Parallel()
	result := ownScan(t)
	seen := map[string]bool{}
	for _, s := range result.Suppressions {
		seen[s.Rule] = true
	}
	for _, rule := range []string{
		"golangci.yml::govet.disable::fieldalignment",
		"golangci.yml::exclusions.rules::gosec/G204",
		"golangci.yml::exclusions.rules::gosec/G306",
	} {
		if !seen[rule] {
			t.Errorf("the detector does not see the configuration exception %s", rule)
		}
	}
	if len(suppression.Check(result, record.Record{}.Matcher())) == 0 {
		t.Error("an empty record licensed the repository's exceptions")
	}
}
