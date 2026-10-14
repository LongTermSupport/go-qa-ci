package runners_test

import (
	"os/exec"
	"testing"

	"github.com/LongTermSupport/go-qa-ci/internal/config"
	"github.com/LongTermSupport/go-qa-ci/internal/runners"
)

// partlyKilled has two covered operators; its test pins only Sum and merely runs Difference, so
// gremlins kills one mutant and two survive: an efficacy of 33.33 percent.
var partlyKilled = map[string]string{
	"pair.go": "package fixture\n\nfunc Sum(a, b int) int {\n\treturn a + b\n}\n\nfunc Difference(a, b int) int {\n\treturn a - b\n}\n",
	"pair_test.go": "package fixture\n\nimport \"testing\"\n\nfunc TestSum(t *testing.T) {\n" +
		"\tif got := Sum(3, 1); got != 4 {\n\t\tt.Fatal(got)\n\t}\n\tif Difference(3, 1) < 0 {\n\t\tt.Fatal(\"negative\")\n\t}\n}\n",
}

// TestMutantsThresholdFailsARealGremlinsRun is the only test that can show the threshold reaches
// gremlins: the fakes exit with whatever code they are told. It needs gremlins on PATH.
func TestMutantsThresholdFailsARealGremlinsRun(t *testing.T) {
	if _, err := exec.LookPath("gremlins"); err != nil {
		t.Skipf("gremlins is not installed: %v", err)
	}
	dir := module(t, partlyKilled)
	for _, tt := range []struct {
		threshold int
		want      bool
	}{
		{100, false},
		{34, false},
		{33, true},
		{0, true},
	} {
		output, passed, err := runners.Mutants(ctx(), dir, runners.MutantsOptions{Mode: config.MutantsFull, ThresholdEfficacy: tt.threshold})
		if err != nil || passed != tt.want {
			t.Errorf("threshold %d: passed %v, err %v, want passed %v\n%s", tt.threshold, passed, err, tt.want, output)
		}
	}
}
