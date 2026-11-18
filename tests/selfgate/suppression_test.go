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

// The repository silences nothing at all: with an empty record its source and configuration yield no
// finding, so its record holds no exception, and its one switch-off (gosec G204, for all code) is a
// choice of the defence set, not a suppression.
func TestOwnSourceAndConfigurationNeedNoException(t *testing.T) {
	t.Parallel()
	result := ownScan(t)
	for _, s := range result.Suppressions {
		t.Errorf("unexpected suppression %s (%s)", s, s.Route)
	}
	for _, f := range suppression.Check(result, record.Record{}.Matcher()) {
		t.Errorf("%s", f)
	}
	if n := len(ownRecord(t).Exceptions); n != 0 {
		t.Errorf("the record holds %d exceptions, want none", n)
	}
}
