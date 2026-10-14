package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/LongTermSupport/go-qa-ci/internal/config"
	"github.com/LongTermSupport/go-qa-ci/internal/defences"
	"github.com/LongTermSupport/go-qa-ci/internal/summary"
	"github.com/LongTermSupport/go-qa-ci/internal/tools"
)

// collect derives the defence listing of the project from its configuration.
func collect(rootPath string, cfg config.Config) (defences.Listing, error) {
	listing, err := defences.Collect(context.Background(), defences.Input{Root: rootPath, Config: cfg, Steps: tools.Steps(cfg)})
	if err != nil {
		return defences.Listing{}, fmt.Errorf("deriving the defence listing: %w", err)
	}
	return listing, nil
}

// runRules prints the defences the active configuration enforces, with the gate's steps and the
// project record against its budget.
func runRules(rootPath string, cfg config.Config, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("go-qa rules", flag.ContinueOnError)
	flags.SetOutput(stderr)
	asJSON := flags.Bool("json", false, "print JSON instead of text")
	if err := flags.Parse(args); err != nil {
		return ExitUsage
	}
	if flags.NArg() > 0 {
		return emit(stderr, ExitUsage, "go-qa rules: unexpected argument %q", flags.Arg(0))
	}
	listing, err := collect(rootPath, cfg)
	if err != nil {
		return emit(stderr, ExitFailure, "go-qa: %v", err)
	}
	if !*asJSON {
		return write(stdout, stderr, renderRules(listing))
	}
	data, err := json.MarshalIndent(listing, "", "  ")
	if err != nil {
		return emit(stderr, ExitFailure, "go-qa: encode the defence listing: %v", err)
	}
	return write(stdout, stderr, string(data)+"\n")
}

func renderRules(l defences.Listing) string {
	var out strings.Builder
	fmt.Fprintf(&out, "%d defences\n", len(l.Defences))
	for _, d := range l.Defences {
		route := d.Doc
		if route == "" {
			route = "(no documentation route)"
		}
		fmt.Fprintf(&out, "  %-9s %-9s %-6s %s  %s  [%s]\n", d.Engine, d.Origin, d.Level, d.ID, d.Description, route)
	}
	for _, group := range []struct {
		title string
		steps []defences.Step
	}{{"stages", l.Stages}, {"runners", l.Runners}} {
		fmt.Fprintf(&out, "%s\n", group.title)
		for _, s := range group.steps {
			state := "enabled"
			if !s.Enabled {
				state = "disabled"
			}
			fmt.Fprintf(&out, "  %-12s %-10s %s\n", s.Name, s.Phase, state)
		}
	}
	fmt.Fprintf(&out, "record: %d exceptions\n", len(l.Record))
	for _, e := range l.Record {
		fmt.Fprintf(&out, "  #%d %s  %s\n      %s\n", e.Number, e.Rule, e.Path, e.Justification)
	}
	out.WriteString("budget\n")
	for _, line := range l.Budget.Lines() {
		fmt.Fprintf(&out, "  %s\n", line)
	}
	return out.String()
}

// runSummary writes the generated region into the configured agent instruction file, or with
// --check only judges it, failing as gqaci.summary.stale when it is out of date.
func runSummary(rootPath string, cfg config.Config, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("go-qa summary", flag.ContinueOnError)
	flags.SetOutput(stderr)
	check := flags.Bool("check", false, "fail when the region is out of date instead of writing it")
	if err := flags.Parse(args); err != nil {
		return ExitUsage
	}
	if flags.NArg() > 0 {
		return emit(stderr, ExitUsage, "go-qa summary: unexpected argument %q", flags.Arg(0))
	}
	file := cfg.Summary.File
	if file == "" {
		return emit(stderr, ExitUsage, "go-qa summary: no file is configured; set summary.file in %s", config.DefaultPath)
	}
	listing, err := collect(rootPath, cfg)
	if err != nil {
		return emit(stderr, ExitFailure, "go-qa: %v", err)
	}
	if !*check {
		if err = summary.Write(rootPath, file, listing); err != nil {
			return emit(stderr, ExitFailure, "go-qa: %v", err)
		}
		return emit(stdout, ExitOK, "summary: wrote the generated region of %s; commit it", file)
	}
	f, stale, err := summary.Check(rootPath, file, listing)
	if err != nil {
		return emit(stderr, ExitFailure, "go-qa: %v", err)
	}
	if stale {
		return emit(stderr, ExitFailure, "%s", f)
	}
	return emit(stdout, ExitOK, "summary: the generated region of %s is current", file)
}
