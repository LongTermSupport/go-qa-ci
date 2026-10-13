package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

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
	return opts, ExitOK, true
}

// ciEnabled reports whether the CI environment variable asks for CI mode: set, and neither
// "false" nor "0".
func ciEnabled() bool {
	v := os.Getenv("CI")
	return v != "" && v != "false" && v != "0"
}

func runPipeline(rootPath string, cfg config.Config, opts runOptions, newExecutor ExecutorFactory, stdout, stderr io.Writer) int {
	steps, err := tools.Select(opts.tools, cfg.Tools)
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
	ctx := tools.Context{
		Root:   rootPath,
		Config: cfg,
		Scope:  scope,
		CI:     opts.ci || ciEnabled(),
		Fix:    !opts.noFix,
		Today:  date.Today(),
	}
	report := pipeline.Execute(steps, newExecutor(ctx, stderr), opts.failFast)
	if _, err := io.WriteString(stdout, summary(report, scope)); err != nil {
		return emit(stderr, ExitFailure, "go-qa: %v", err)
	}
	if !report.Passed {
		return ExitFailure
	}
	return ExitOK
}

// summary renders the run summary table, the stop notice and, on failure, the DBF pointer.
func summary(report pipeline.Report, scope []string) string {
	var out strings.Builder
	out.WriteString("\ngo-qa summary:\n")
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
