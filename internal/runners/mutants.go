package runners

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/LongTermSupport/go-qa-ci/internal/config"
)

const gremlins = "gremlins"

var errNotRun = errors.New("mutants mode off is not run")

// MutantsOptions is how mutation testing is run.
type MutantsOptions struct {
	// Mode is diff or full; off never reaches this package.
	Mode config.MutantsMode
	// Base is the git ref diff mode compares against.
	Base string
	// ThresholdEfficacy is the least percentage of mutants the tests must kill.
	ThresholdEfficacy int
}

// efficacyMargin turns "at least N percent" into gremlins' "fail when efficacy <= threshold": the
// threshold written is N less this margin, which no real efficacy (a ratio of mutant counts)
// falls inside.
const efficacyMargin = 0.0001

// timeoutCoefficient is how many times the baseline test time a mutant's test run may take before
// gremlins calls it timed out. A timed-out mutant is not counted as killed, so the default of 3
// turns a loaded machine (a mutant needs a fresh compile) into a false failure.
const timeoutCoefficient = 10

// MutantsConfig is the gremlins configuration file holding the efficacy threshold and the timeout
// coefficient. gremlins v0.6.0
// ignores `--threshold-efficacy` (viper hands the float flag back as a string, so the value reads
// as 0 and no gate applies) but honours the same key from a configuration file. gremlins fails when
// efficacy <= threshold and treats 0 as no gate, so a threshold of N percent is written as N less
// efficacyMargin, and a threshold of 0 stays 0.
func MutantsConfig(thresholdEfficacy int) string {
	threshold := 0.0
	if thresholdEfficacy > 0 {
		threshold = float64(thresholdEfficacy) - efficacyMargin
	}
	return "unleash:\n  timeout-coefficient: " + strconv.Itoa(timeoutCoefficient) +
		"\n  threshold:\n    efficacy: " + strconv.FormatFloat(threshold, 'f', -1, 64) + "\n"
}

// MutantsArgs are the arguments of `gremlins unleash` for the options, reading the threshold from
// the configuration file at configPath. Diff mode restricts the mutants to the lines changed since
// Base (`--diff`); both modes fail below the efficacy threshold, because gremlins otherwise exits 0
// whatever survives.
func MutantsArgs(opts MutantsOptions, configPath string) ([]string, error) {
	args := []string{"unleash", "--config", configPath}
	switch opts.Mode {
	case config.MutantsDiff:
		if opts.Base == "" {
			return nil, fmt.Errorf("mutants mode diff needs a base ref")
		}
		args = append(args, "--diff", opts.Base)
	case config.MutantsFull:
	case config.MutantsOff:
		return nil, errNotRun
	default:
		return nil, fmt.Errorf("mutants mode %q is unknown", opts.Mode)
	}
	return args, nil
}

// Mutants runs gremlins over the module in dir. passed is false when the tests killed too few
// mutants (or gremlins failed); output is what it printed.
func Mutants(ctx context.Context, dir string, opts MutantsOptions) (output string, passed bool, err error) {
	configDir, err := os.MkdirTemp("", "go-qa-gremlins-")
	if err != nil {
		return "", false, fmt.Errorf("gremlins config directory: %w", err)
	}
	defer func() { err = errors.Join(err, os.RemoveAll(configDir)) }()
	configPath := filepath.Join(configDir, "gremlins.yaml")
	if writeErr := os.WriteFile(configPath, []byte(MutantsConfig(opts.ThresholdEfficacy)), 0o600); writeErr != nil {
		return "", false, fmt.Errorf("write %s: %w", configPath, writeErr)
	}
	args, err := MutantsArgs(opts, configPath)
	if err != nil {
		return "", false, err
	}
	if locateErr := Locate(gremlins); locateErr != nil {
		return "", false, locateErr
	}
	out, err := Capture(ctx, dir, gremlins, args...)
	if err != nil {
		return "", false, err
	}
	return out.Combined(), out.Code == 0, nil
}
