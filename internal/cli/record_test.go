package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	dir := module(t, "")
	writeFile(t, dir, "qa/record.toml", recordToml)
	return dir
}

func validRecord() string {
	return "[[exception]]\nrule = \"errcheck\"\npath = \"a_test.go\"\njustification = \"" + goodWhy + "\"\n"
}

func TestRecordCheckPassesAValidRecord(t *testing.T) {
	r := exec("--root", recordModule(t, validRecord()), "record", "check")
	if r.code != 0 || r.err != "" || !strings.Contains(r.out, "1 exceptions") {
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
