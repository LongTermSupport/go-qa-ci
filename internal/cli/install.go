package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strings"

	goqaci "github.com/LongTermSupport/go-qa-ci"
	"github.com/LongTermSupport/go-qa-ci/internal/toolstore"
)

// Toolchain is the store of pinned external tools: what `go-qa install-tools` fills and what the
// gate resolves its tools from.
type Toolchain interface {
	// Names are the pinned tools, sorted.
	Names() []string
	// Install installs the named tools (every pinned tool when none are named).
	Install(ctx context.Context, names ...string) error
	// Ensure installs what is missing, then puts the pinned copies first on PATH.
	Ensure(ctx context.Context, names ...string) error
	// Missing lists the pinned tools that are not installed.
	Missing() []string
	// Resolve is the path of an installed pinned tool.
	Resolve(name string) (string, bool)
}

// ToolchainFactory builds the Toolchain of one invocation; install progress goes to log.
type ToolchainFactory func(log io.Writer) (Toolchain, error)

// DefaultToolchain is the store under the user's cache directory (or $GOQA_TOOLS_DIR), holding
// the pins of the tools.lock this binary was built with.
func DefaultToolchain(log io.Writer) (Toolchain, error) {
	lock, err := toolstore.ParseLock(goqaci.ToolsLock)
	if err != nil {
		return nil, fmt.Errorf("the tools.lock built into go-qa: %w", err)
	}
	dir, err := toolstore.DefaultDir()
	if err != nil {
		return nil, fmt.Errorf("locating the tool directory: %w", err)
	}
	return toolstore.New(lock, dir, log), nil
}

// ensureTools installs the named pinned tools that are missing and puts them first on PATH. A
// nil factory means the caller supplies its own tools.
func ensureTools(ctx context.Context, newToolchain ToolchainFactory, names []string, stderr io.Writer) error {
	if newToolchain == nil || len(names) == 0 {
		return nil // an empty list would mean every pinned tool to the store
	}
	chain, err := newToolchain(stderr)
	if err != nil {
		return err
	}
	if err = chain.Ensure(ctx, names...); err != nil {
		return fmt.Errorf("the pinned tools: %w", err)
	}
	return nil
}

// runInstallTools is `go-qa install-tools [--check] [--print-path] [tool...]`.
func runInstallTools(args []string, newToolchain ToolchainFactory, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("go-qa install-tools", flag.ContinueOnError)
	flags.SetOutput(stderr)
	check := flags.Bool("check", false, "install nothing; report the pinned tools that are missing and exit 1 if any")
	printPath := flags.Bool("print-path", false, "after installing, print the directory of each tool, one per line")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return ExitOK
		}
		return ExitUsage
	}
	if *check && *printPath {
		return emit(stderr, ExitUsage, "go-qa install-tools: --check installs nothing, so --print-path has nothing to print; use one or the other")
	}
	if newToolchain == nil {
		return emit(stderr, ExitUsage, "go-qa install-tools: no tool store is configured")
	}
	chain, err := newToolchain(stderr)
	if err != nil {
		return emit(stderr, ExitFailure, "go-qa install-tools: %v", err)
	}
	names := flags.Args()
	if unknown := unpinned(chain.Names(), names); unknown != "" {
		return emit(stderr, ExitUsage, "go-qa install-tools: %s is not pinned in tools.lock (pinned: %s)", unknown, strings.Join(chain.Names(), ", "))
	}
	if *check {
		return reportMissing(chain, names, stderr)
	}
	if err = chain.Install(context.Background(), names...); err != nil {
		return emit(stderr, ExitFailure, "go-qa install-tools: %v", err)
	}
	if *printPath {
		return printDirs(chain, names, stdout, stderr)
	}
	return ExitOK
}

func unpinned(pinned, names []string) string {
	for _, name := range names {
		if !slices.Contains(pinned, name) {
			return name
		}
	}
	return ""
}

func reportMissing(chain Toolchain, names []string, stderr io.Writer) int {
	code := ExitOK
	for _, name := range chain.Missing() {
		if len(names) > 0 && !slices.Contains(names, name) {
			continue
		}
		code = emit(stderr, ExitFailure, "go-qa install-tools: %s is not installed (run go-qa install-tools)", name)
	}
	return code
}

func printDirs(chain Toolchain, names []string, stdout, stderr io.Writer) int {
	if len(names) == 0 {
		names = chain.Names()
	}
	for _, name := range names {
		path, ok := chain.Resolve(name)
		if !ok {
			return emit(stderr, ExitFailure, "go-qa install-tools: %s is not installed", name)
		}
		if _, err := fmt.Fprintln(stdout, filepath.Dir(path)); err != nil {
			return emit(stderr, ExitFailure, "go-qa install-tools: %v", err)
		}
	}
	return ExitOK
}
