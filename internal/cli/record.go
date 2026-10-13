package cli

import (
	"flag"
	"io"
	"strconv"

	"github.com/LongTermSupport/go-qa-ci/internal/config"
	"github.com/LongTermSupport/go-qa-ci/internal/date"
	"github.com/LongTermSupport/go-qa-ci/internal/record"
)

const recordUsage = "usage: go-qa record check | list [--json] | explain <rule|number> | prune <number>..."

// runRecord is the record subcommand: check validates, list enumerates, explain shows one entry.
func runRecord(rootPath string, cfg config.Config, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return emit(stderr, ExitUsage, "%s", recordUsage)
	}
	rest := args[1:]
	switch args[0] {
	case "check":
		if len(rest) > 0 {
			return emit(stderr, ExitUsage, "go-qa record check: unexpected argument %q", rest[0])
		}
		return recordCheck(rootPath, cfg, stdout, stderr)
	case "list":
		return recordList(rootPath, cfg, rest, stdout, stderr)
	case "explain":
		if len(rest) != 1 {
			return emit(stderr, ExitUsage, "go-qa record explain: want exactly one rule or number\n%s", recordUsage)
		}
		return recordExplain(rootPath, cfg, rest[0], stdout, stderr)
	case "prune":
		return recordPrune(rootPath, cfg, rest, stdout, stderr)
	default:
		return emit(stderr, ExitUsage, "go-qa record: unknown command %q\n%s", args[0], recordUsage)
	}
}

func recordCheck(rootPath string, cfg config.Config, stdout, stderr io.Writer) int {
	rec, findings, err := record.Check(rootPath, cfg.Record, date.Today())
	if err != nil {
		return emit(stderr, ExitUsage, "go-qa: %v", err)
	}
	for _, f := range findings {
		if code := emit(stderr, ExitFailure, "%s", f); code != ExitFailure {
			return code
		}
	}
	if len(findings) > 0 {
		return ExitFailure
	}
	return emit(stdout, ExitOK, "record: %d exceptions in %s, all valid", len(rec.Exceptions), cfg.Record.Path)
}

func recordList(rootPath string, cfg config.Config, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("go-qa record list", flag.ContinueOnError)
	flags.SetOutput(stderr)
	asJSON := flags.Bool("json", false, "print JSON instead of text")
	if err := flags.Parse(args); err != nil {
		return ExitUsage
	}
	if flags.NArg() > 0 {
		return emit(stderr, ExitUsage, "go-qa record list: unexpected argument %q", flags.Arg(0))
	}
	rec, _, err := record.Check(rootPath, cfg.Record, date.Today())
	if err != nil {
		return emit(stderr, ExitUsage, "go-qa: %v", err)
	}
	if !*asJSON {
		return write(stdout, stderr, rec.List())
	}
	data, err := rec.ListJSON()
	if err != nil {
		return emit(stderr, ExitFailure, "go-qa: %v", err)
	}
	return write(stdout, stderr, string(data)+"\n")
}

func recordExplain(rootPath string, cfg config.Config, selector string, stdout, stderr io.Writer) int {
	rec, _, err := record.Check(rootPath, cfg.Record, date.Today())
	if err != nil {
		return emit(stderr, ExitUsage, "go-qa: %v", err)
	}
	text, err := rec.Explain(selector)
	if err != nil {
		return emit(stderr, ExitUsage, "go-qa record explain: %v", err)
	}
	return write(stdout, stderr, text)
}

// recordPrune removes the exceptions numbered in args (1-based, as record list shows them) and
// lowers the ratchet to what remains.
func recordPrune(rootPath string, cfg config.Config, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return emit(stderr, ExitUsage, "go-qa record prune: want one or more exception numbers\n%s", recordUsage)
	}
	indexes := make([]int, 0, len(args))
	for _, arg := range args {
		number, err := strconv.Atoi(arg)
		if err != nil || number < 1 {
			return emit(stderr, ExitUsage, "go-qa record prune: %q is not an exception number (1 or more)", arg)
		}
		indexes = append(indexes, number-1)
	}
	remaining, err := record.Prune(rootPath, cfg.Record.Path, indexes)
	if err != nil {
		return emit(stderr, ExitUsage, "go-qa record prune: %v", err)
	}
	return emit(stdout, ExitOK, "removed %d exception(s); %d remain; commit the change", len(indexes), remaining)
}

// write sends text to stdout; a failed write is a failure of the command.
func write(stdout, stderr io.Writer, text string) int {
	if _, err := io.WriteString(stdout, text); err != nil {
		return emit(stderr, ExitFailure, "go-qa: %v", err)
	}
	return ExitOK
}
