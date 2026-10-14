package runners_test

import (
	"strings"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/runners"
)

func TestModAcceptsATidyModule(t *testing.T) {
	problems, err := runners.Mod(ctx(), module(t, map[string]string{"main.go": "package main\n\nfunc main() {}\n"}))
	if err != nil || len(problems) != 0 {
		t.Errorf("got %v, %v", problems, err)
	}
}

func TestModReportsAnUntidyGoMod(t *testing.T) {
	dir := module(t, map[string]string{
		"main.go":    "package main\n\nfunc main() {}\n",
		"go.mod":     "module example.com/fixture\n\ngo 1.26\n\nrequire example.com/dep v0.0.0\n\nreplace example.com/dep => ./dep\n",
		"dep/go.mod": "module example.com/dep\n\ngo 1.26\n",
	})
	problems, err := runners.Mod(ctx(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 1 || !strings.Contains(problems[0], "not tidy") || !strings.Contains(problems[0], "-require example.com/dep") && !strings.Contains(problems[0], "example.com/dep") {
		t.Errorf("got %q", problems)
	}
}

func TestModFailsWhenThereIsNoModule(t *testing.T) {
	if _, err := runners.Mod(ctx(), t.TempDir()); err == nil {
		t.Error("a directory without go.mod passed")
	}
}
