// Package cli is the go-qa command line: global flags, then a subcommand.
package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"strings"

	"github.com/LongTermSupport/go-qa-ci/internal/config"
	"github.com/LongTermSupport/go-qa-ci/internal/pipeline"
	"github.com/LongTermSupport/go-qa-ci/internal/root"
	"github.com/LongTermSupport/go-qa-ci/internal/tools"
)

// DevVersion is what go-qa reports when it was not built from a tagged module version.
const DevVersion = "0.0.0-dev"

// Version is the go-qa version: the module version the binary was built from (a tag, through
// `go install ...@<tag>`), else DevVersion.
var Version = currentVersion()

func currentVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return DevVersion
	}
	return VersionFrom(info)
}

// VersionFrom is the module version recorded in info, or DevVersion when there is none or the
// binary was built from a checkout ("(devel)").
func VersionFrom(info *debug.BuildInfo) string {
	if info == nil || info.Main.Version == "" || info.Main.Version == "(devel)" {
		return DevVersion
	}
	return info.Main.Version
}

// Exit codes: 0 success, 1 a defence or tool failed, 2 usage or configuration error.
const (
	ExitOK      = 0
	ExitFailure = 1
	ExitUsage   = 2
)

const usage = "usage: go-qa [--root DIR] [--config FILE] tools | run [-t TOOL]... [-p PATH]... [--ci] [--no-fix] [--fail-fast] [--changed [--base REF]] | record check|list|explain | rule-doc ID | rules [--json] | summary [--check] | docs-check | go-qa --version"

// ExecutorFactory builds the executor for one run from its context.
type ExecutorFactory func(ctx tools.Context, stderr io.Writer) pipeline.Executor

// Run executes go-qa with args (without the program name) and returns the exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	return RunWith(args, stdout, stderr, func(ctx tools.Context, errOut io.Writer) pipeline.Executor {
		return tools.NewExecutor(ctx, errOut)
	})
}

// RunWith is Run with the executor of the `run` subcommand supplied by the caller.
func RunWith(args []string, stdout, stderr io.Writer, newExecutor ExecutorFactory) int {
	flags := flag.NewFlagSet("go-qa", flag.ContinueOnError)
	flags.SetOutput(stderr)
	rootDir := flags.String("root", "", "project root (default: the go.work or go.mod directory)")
	configFile := flags.String("config", "", "configuration file relative to the root (default: "+config.DefaultPath+")")
	showVersion := flags.Bool("version", false, "print the version")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return ExitOK
		}
		return ExitUsage
	}
	if *showVersion {
		return emit(stdout, ExitOK, "%s", Version)
	}
	rest := flags.Args()
	if len(rest) == 0 {
		return emit(stderr, ExitUsage, "%s", usage)
	}
	switch rest[0] {
	case "tools":
		if len(rest) > 1 {
			return emit(stderr, ExitUsage, "go-qa tools: unexpected argument %q", rest[1])
		}
		return withConfig(*rootDir, *configFile, stderr, func(_ string, cfg config.Config) int {
			return listTools(cfg, stdout, stderr)
		})
	case "run":
		opts, code, ok := parseRun(rest[1:], stderr)
		if !ok {
			return code
		}
		return withConfig(*rootDir, *configFile, stderr, func(rootPath string, cfg config.Config) int {
			return runPipeline(rootPath, *configFile, cfg, opts, newExecutor, stdout, stderr)
		})
	case "record":
		return withConfig(*rootDir, *configFile, stderr, func(rootPath string, cfg config.Config) int {
			return runRecord(rootPath, cfg, rest[1:], stdout, stderr)
		})
	case "rule-doc":
		return withConfig(*rootDir, *configFile, stderr, func(rootPath string, cfg config.Config) int {
			return runRuleDoc(rootPath, cfg, rest[1:], stdout, stderr)
		})
	case "rules":
		return withConfig(*rootDir, *configFile, stderr, func(rootPath string, cfg config.Config) int {
			return runRules(rootPath, cfg, rest[1:], stdout, stderr)
		})
	case "summary":
		return withConfig(*rootDir, *configFile, stderr, func(rootPath string, cfg config.Config) int {
			return runSummary(rootPath, cfg, rest[1:], stdout, stderr)
		})
	case "docs-check":
		return withConfig(*rootDir, *configFile, stderr, func(rootPath string, cfg config.Config) int {
			return runDocsCheck(rootPath, cfg, rest[1:], stdout, stderr)
		})
	default:
		return emit(stderr, ExitUsage, "go-qa: unknown subcommand %q", rest[0])
	}
}

// withConfig resolves the root, loads the configuration and runs fn; any failure to do so is a
// usage error.
func withConfig(rootDir, configFile string, stderr io.Writer, fn func(string, config.Config) int) int {
	if rootDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return emit(stderr, ExitUsage, "go-qa: %v", err)
		}
		if rootDir, err = root.Discover(context.Background(), cwd); err != nil {
			return emit(stderr, ExitUsage, "go-qa: %v", err)
		}
	}
	cfg, err := config.Load(rootDir, configFile)
	if err != nil {
		return emit(stderr, ExitUsage, "go-qa: %v", err)
	}
	if err = tools.CheckStages(cfg); err != nil {
		return emit(stderr, ExitUsage, "go-qa: %v", err)
	}
	return fn(rootDir, cfg)
}

func listTools(cfg config.Config, stdout, stderr io.Writer) int {
	var out strings.Builder
	for _, tool := range tools.Catalogue() {
		state := "disabled"
		if tool.Enabled(cfg.Tools) {
			state = "enabled"
		}
		fmt.Fprintf(&out, "%-12s %-10s %s\n", tool.Name, tool.Phase, state)
	}
	for _, s := range cfg.Stages {
		fmt.Fprintf(&out, "%-12s %-10s stage\n", s.Name, s.Phase)
	}
	if _, err := io.WriteString(stdout, out.String()); err != nil {
		return emit(stderr, ExitFailure, "go-qa: %v", err)
	}
	return ExitOK
}

// emit writes one formatted line to w and returns code, or ExitFailure when the write fails.
func emit(w io.Writer, code int, format string, args ...any) int {
	if _, err := fmt.Fprintf(w, format+"\n", args...); err != nil {
		return ExitFailure
	}
	return code
}
