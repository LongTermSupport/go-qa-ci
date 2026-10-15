package toolstore_test

import (
	"os"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/faketool"
)

func TestMain(m *testing.M) {
	if code, served := faketool.Main(); served {
		os.Exit(code)
	}
	os.Exit(m.Run())
}
