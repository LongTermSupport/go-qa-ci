package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/date"
)

const goodWhy = "Tests assert on panics deliberately; a panic is the failure signal in test code."

type result struct {
	code int
	out  string
	err  string
}

func exec(args ...string) result {
	code, out, errOut := run(args...)
	return result{code: code, out: out, err: errOut}
}

func writeFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	path := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func recordModule(t *testing.T, recordToml string) string {
	t.Helper()
	dir := module(t, "[record]\ndeciders = [\"alice\"]\n")
	writeFile(t, dir, "qa/record.toml", recordToml)
	return dir
}

// validRecord is decided today by a configured decider and due for review in 30 days; the CLI
// reads the real clock, so the dates are relative to it.
func validRecord() string {
	today := date.Today()
	return "[[exception]]\nrule = \"errcheck\"\npath = \"a_test.go\"\njustification = \"" + goodWhy + "\"\n" +
		"decided_by = \"alice\"\nadded_by_model = \"human\"\n" +
		"decided_on = " + today.String() + "\nreview_by = " + today.AddDays(30).String() + "\n"
}

func TestRecordCheckReportsAnExpiredEntry(t *testing.T) {
	old := "[[exception]]\nrule = \"errcheck\"\npath = \"a_test.go\"\njustification = \"" + goodWhy + "\"\n" +
		"decided_by = \"alice\"\nadded_by_model = \"human\"\ndecided_on = 2020-01-01\nreview_by = 2020-02-01\n"
	r := exec("--root", recordModule(t, old), "record", "check")
	if r.code != 1 || !strings.Contains(r.err, "error[gqaci.record.expired]") {
		t.Errorf("got %+v", r)
	}
}

func TestRecordCheckReportsAnOverBudgetRecord(t *testing.T) {
	root := module(t, "[record]\ndeciders = [\"alice\"]\nmax_total = 1\nmax_per_rule = 1\n")
	writeFile(t, root, "qa/record.toml", validRecord()+"\n"+validRecord())
	r := exec("--root", root, "record", "check")
	if r.code != 1 || !strings.Contains(r.err, "error[gqaci.record.budget]") {
		t.Errorf("got %+v", r)
	}
}

func TestRecordCheckPassesAValidRecord(t *testing.T) {
	signed := "[ratchet]\nhigh_water = 1\ndecided_by = \"alice\"\ndecided_on = " + date.Today().String() +
		"\nreason = \"" + goodWhy + "\"\n\n"
	r := exec("--root", recordModule(t, signed+validRecord()), "record", "check")
	if r.code != 0 || !strings.HasPrefix(r.err, "note: no committed history") || !strings.Contains(r.out, "1 exceptions") {
		t.Errorf("got %+v", r)
	}
}

func TestRecordCheckPassesWhenThereIsNoRecord(t *testing.T) {
	r := exec("--root", module(t, ""), "record", "check")
	if r.code != 0 || r.err != "" {
		t.Errorf("got %+v", r)
	}
}

func TestRecordCheckRejectsGenericJustification(t *testing.T) {
	bad := "[[exception]]\nrule = \"errcheck\"\npath = \"a.go\"\njustification = \"legacy\"\n"
	r := exec("--root", recordModule(t, bad), "record", "check")
	for _, want := range []string{"error[gqaci.record.invalid]", "generic", "qa/record.toml:1"} {
		if r.code != 1 || !strings.Contains(r.err, want) {
			t.Errorf("want %q: got %+v", want, r)
		}
	}
}

func TestRecordCheckRejectsGlobWithoutPolicy(t *testing.T) {
	bad := "[[exception]]\nrule = \"errcheck\"\npath = \"**/a.go\"\njustification = \"" + goodWhy + "\"\n"
	r := exec("--root", recordModule(t, bad), "record", "check")
	if r.code != 1 || !strings.Contains(r.err, "policy = true") {
		t.Errorf("got %+v", r)
	}
}

func TestRecordCheckMalformedFileIsAUsageLevelError(t *testing.T) {
	r := exec("--root", recordModule(t, "[[exception]]\nreson = 1\n"), "record", "check")
	if r.code != 2 || !strings.Contains(r.err, "reson") || !strings.Contains(r.err, "qa/record.toml") {
		t.Errorf("got %+v", r)
	}
}

func TestRecordHonoursTheConfiguredPath(t *testing.T) {
	dir := module(t, "record = \"decisions/exceptions.toml\"\n")
	writeFile(t, dir, "decisions/exceptions.toml", validRecord())
	r := exec("--root", dir, "record", "list")
	if r.code != 0 || r.err != "" || !strings.Contains(r.out, "#1 errcheck") {
		t.Errorf("got %+v", r)
	}
}

func TestRecordListShowsJustifications(t *testing.T) {
	r := exec("--root", recordModule(t, validRecord()), "record", "list")
	if r.code != 0 || r.err != "" || !strings.Contains(r.out, goodWhy) {
		t.Errorf("got %+v", r)
	}
}

func TestRecordListJSON(t *testing.T) {
	r := exec("--root", recordModule(t, validRecord()), "record", "list", "--json")
	var decoded struct {
		Exceptions []struct{ Rule string } `json:"exceptions"`
	}
	err := json.Unmarshal([]byte(r.out), &decoded)
	if r.code != 0 || err != nil || len(decoded.Exceptions) != 1 || decoded.Exceptions[0].Rule != "errcheck" {
		t.Errorf("got %+v, %v", r, err)
	}
}

func TestRecordListStillListsAnInvalidRecord(t *testing.T) {
	bad := "[[exception]]\nrule = \"errcheck\"\npath = \"a.go\"\njustification = \"legacy\"\n"
	r := exec("--root", recordModule(t, bad), "record", "list")
	if r.code != 0 || !strings.Contains(r.out, "legacy") {
		t.Errorf("got %+v", r)
	}
}

func TestRecordExplain(t *testing.T) {
	root := recordModule(t, validRecord())
	if r := exec("--root", root, "record", "explain", "errcheck"); r.code != 0 || !strings.Contains(r.out, "a_test.go") {
		t.Errorf("by rule: %+v", r)
	}
	if r := exec("--root", root, "record", "explain", "1"); r.code != 0 || !strings.Contains(r.out, goodWhy) {
		t.Errorf("by number: %+v", r)
	}
	if r := exec("--root", root, "record", "explain", "nope"); r.code != 2 || !strings.Contains(r.err, "nope") {
		t.Errorf("unknown: %+v", r)
	}
}

func TestRecordUsageErrors(t *testing.T) {
	root := module(t, "")
	for _, args := range [][]string{
		{"record"},
		{"record", "bogus"},
		{"record", "check", "extra"},
		{"record", "list", "--bogus"},
		{"record", "list", "extra"},
		{"record", "explain"},
		{"record", "explain", "a", "b"},
	} {
		r := exec(append([]string{"--root", root}, args...)...)
		if r.code != 2 || r.err == "" {
			t.Errorf("%v: got %+v", args, r)
		}
	}
}

func TestRecordPruneRemovesNamedEntriesAndLowersTheCeiling(t *testing.T) {
	signed := "[ratchet]\nhigh_water = 2\ndecided_by = \"alice\"\ndecided_on = " + date.Today().String() +
		"\nreason = \"" + goodWhy + "\"\n\n"
	root := recordModule(t, signed+validRecord()+"\n"+strings.Replace(validRecord(), "a_test.go", "b_test.go", 1))
	r := exec("--root", root, "record", "prune", "1")
	if r.code != 0 || !strings.Contains(r.out, "1 remain") {
		t.Fatalf("got %+v", r)
	}
	data := readRecord(t, root)
	if !strings.Contains(string(data), "high_water = 1") || strings.Contains(string(data), "a_test.go") {
		t.Errorf("record not pruned:\n%s", data)
	}
}

func TestRecordPruneRejectsBadArguments(t *testing.T) {
	root := recordModule(t, validRecord())
	for _, args := range [][]string{{"record", "prune"}, {"record", "prune", "x"}, {"record", "prune", "0"}, {"record", "prune", "9"}} {
		if r := exec(append([]string{"--root", root}, args...)...); r.code != 2 {
			t.Errorf("%v: got %+v", args, r)
		}
	}
}

func readRecord(t *testing.T, root string) []byte {
	t.Helper()
	dir, err := os.OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := dir.Close(); closeErr != nil {
			t.Error(closeErr)
		}
	}()
	data, err := dir.ReadFile("qa/record.toml")
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestRecordCheckSaysWhenTheRatchetCouldNotBeApplied(t *testing.T) {
	r := exec("--root", recordModule(t, validRecord()), "record", "check")
	if !strings.Contains(r.err, "no committed history") {
		t.Errorf("the missing history must be reported, got %+v", r)
	}
}
