package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/LongTermSupport/go-qa-ci/internal/changed"
	"github.com/LongTermSupport/go-qa-ci/internal/config"
	"github.com/LongTermSupport/go-qa-ci/internal/date"
	"github.com/LongTermSupport/go-qa-ci/internal/pipeline"
	"github.com/LongTermSupport/go-qa-ci/internal/root"
	"github.com/LongTermSupport/go-qa-ci/internal/tools"
)

// DBFPointer is printed after a failing run: failures name a rule, the method explains why.
const DBFPointer = "Defence Before Fix: each failure names an identifier; fix the class of defect, not only the instance.\n" +
	"Method: https://defence-before-fix.github.io/"

// listFlag is a repeatable string flag.
type listFlag []string

func (l *listFlag) String() string { return strings.Join(*l, ",") }

func (l *listFlag) Set(v string) error {
	*l = append(*l, v)
	return nil
}

// runOptions are the parsed flags of the run subcommand.
type runOptions struct {
	tools    listFlag
	paths    listFlag
	ci       bool
	noFix    bool
	failFast bool
	changed  bool
	base     string
}

// parseRun parses the run flags; when ok is false the caller returns code.
func parseRun(args []string, stderr io.Writer) (opts runOptions, code int, ok bool) {
	flags := flag.NewFlagSet("go-qa run", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Var(&opts.tools, "t", "run only this tool (repeatable); default: every enabled tool")
	flags.Var(&opts.paths, "p", "limit the run to this file or directory (repeatable)")
	flags.BoolVar(&opts.ci, "ci", false, "CI mode: autofix, then fail if the tree changed (implied by CI)")
	flags.BoolVar(&opts.noFix, "no-fix", false, "check formatting without modifying files")
	flags.BoolVar(&opts.failFast, "fail-fast", false, "stop at the first failure")
	flags.BoolVar(&opts.changed, "changed", false, "diff mode, for fast local iteration (the full run stays the gate before a push): "+
		"per-package engines run on the changed packages and their importers, file-scoped detectors on the changed files, "+
		"the repo-wide defences always; changed = files changed against HEAD plus untracked files that are not ignored; "+
		"a change to go.mod, go.sum, the golangci configuration, qa/ or the tool lock widens the run to everything")
	flags.StringVar(&opts.base, "base", "", "with --changed: compare with the merge base of this ref and HEAD (plus the working tree) instead of HEAD")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return opts, ExitOK, false
		}
		// flag has already described the failure on stderr.
		return opts, ExitUsage, false
	}
	if flags.NArg() > 0 {
		return opts, emit(stderr, ExitUsage, "go-qa run: unexpected argument %q", flags.Arg(0)), false
	}
	switch {
	case opts.base != "" && !opts.changed:
		return opts, emit(stderr, ExitUsage, "go-qa run: --base needs --changed"), false
	case opts.changed && len(opts.paths) > 0:
		return opts, emit(stderr, ExitUsage, "go-qa run: --changed chooses the scope itself; it cannot be combined with -p"), false
	}
	return opts, ExitOK, true
}

// ciEnabled reports whether the CI environment variable asks for CI mode: set, and neither
// "false" nor "0".
func ciEnabled() bool {
	v := os.Getenv("CI")
	return v != "" && v != "false" && v != "0"
}

func runPipeline(rootPath, configFile string, cfg config.Config, opts runOptions, newExecutor ExecutorFactory, stdout, stderr io.Writer) int {
	steps, err := tools.Select(opts.tools, cfg)
	if err != nil {
		return emit(stderr, ExitUsage, "go-qa: %v", err)
	}
	scope := make([]string, 0, len(opts.paths))
	for _, path := range opts.paths {
		rel, err := root.Relative(rootPath, path)
		if err != nil {
			return emit(stderr, ExitUsage, "go-qa: %v", err)
		}
		scope = append(scope, rel)
	}
	var diff *changed.Plan
	if opts.changed {
		configRel, err := config.RootRelative(rootPath, configFile)
		if err != nil {
			return emit(stderr, ExitUsage, "go-qa: %v", err)
		}
		diff, err = changed.Compute(context.Background(), changed.Options{
			Root:    rootPath,
			Base:    opts.base,
			Modules: append([]string{""}, cfg.Modules...),
			Widen:   []string{cfg.Record.Path, configRel},
		})
		if err != nil {
			return emit(stderr, ExitUsage, "go-qa: --changed: %v", err)
		}
	}
	ctx := tools.Context{
		Root:   rootPath,
		Config: cfg,
		Scope:  scope,
		CI:     opts.ci || ciEnabled(),
		Fix:    !opts.noFix,
		Today:  date.Today(),
		Diff:   diff,
	}
	report := pipeline.Execute(steps, newExecutor(ctx, stderr), opts.failFast)
	if _, err := io.WriteString(stdout, renderRunSummary(report, scope, diff)); err != nil {
		return emit(stderr, ExitFailure, "go-qa: %v", err)
	}
	if !report.Passed {
		return ExitFailure
	}
	return ExitOK
}

// renderRunSummary renders the run summary table, the stop notice and, on failure, the DBF pointer.
func renderRunSummary(report pipeline.Report, scope []string, diff *changed.Plan) string {
	var out strings.Builder
	out.WriteString("\ngo-qa summary:\n")
	if diff != nil {
		out.WriteString(diff.Describe())
	}
	if len(scope) > 0 {
		fmt.Fprintf(&out, "scope: %s\n", strings.Join(scope, ", "))
	}
	for _, result := range report.Results {
		text := "passed"
		switch result.Outcome.Kind {
		case pipeline.KindFailed:
			text = "FAILED"
			if result.Outcome.Reason != "" {
				text += " (" + result.Outcome.Reason + ")"
			}
		case pipeline.KindSkipped:
			text = "skipped (" + result.Outcome.Reason + ")"
		case pipeline.KindPassed:
		}
		fmt.Fprintf(&out, "  %-10s %-12s %s\n", result.Step.Phase, result.Step.Name, text)
	}
	if report.StoppedBeforeRunners {
		out.WriteString("detectors failed: runners were not started\n")
	}
	if !report.Passed {
		out.WriteString("\n" + DBFPointer + "\n")
	}
	return out.String()
}
