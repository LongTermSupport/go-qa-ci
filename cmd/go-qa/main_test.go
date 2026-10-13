package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantOut    string
		wantErrSub string
	}{
		{"version", []string{"--version"}, 0, version + "\n", ""},
		{"unknown subcommand", []string{"bogus"}, 2, "", "unknown subcommand"},
		{"no arguments", nil, 2, "", "usage"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			code := run(tc.args, &out, &errOut)
			if code != tc.wantCode {
				t.Errorf("exit code = %d, want %d", code, tc.wantCode)
			}
			if out.String() != tc.wantOut {
				t.Errorf("stdout = %q, want %q", out.String(), tc.wantOut)
			}
			if !strings.Contains(errOut.String(), tc.wantErrSub) {
				t.Errorf("stderr = %q, want substring %q", errOut.String(), tc.wantErrSub)
			}
		})
	}
}
