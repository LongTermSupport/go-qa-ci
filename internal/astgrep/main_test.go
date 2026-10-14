package astgrep_test

import (
	"os"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/astgreptest"
)

// TestMain lets this test binary stand in for the ast-grep executable (see astgreptest.Install).
func TestMain(m *testing.M) {
	if code, served := astgreptest.Serve(); served {
		os.Exit(code)
	}
	os.Exit(m.Run())
}
