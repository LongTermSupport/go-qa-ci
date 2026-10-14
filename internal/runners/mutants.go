package runners

import (
	"context"
	"errors"
	"fmt"
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

// MutantsArgs are the arguments of `gremlins unleash` for the options. Diff mode restricts the
// mutants to the lines changed since Base (`--diff`); both modes fail below the efficacy
// threshold, because gremlins otherwise exits 0 whatever survives.
func MutantsArgs(opts MutantsOptions) ([]string, error) {
	args := []string{"unleash", "--threshold-efficacy", strconv.Itoa(opts.ThresholdEfficacy)}
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
	args, err := MutantsArgs(opts)
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
