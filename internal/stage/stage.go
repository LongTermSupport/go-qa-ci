// Package stage is the project's own defences and runners: commands a project declares in
// qa/go-qa.toml, each run in one phase of the gate, for what no built-in tool covers (another
// language's formatter or linter, a lint library's own suite, a check that a generated file is
// fresh). A stage is part of the project's Toolchain: it is listed, documented under its name and,
// when it reports findings, subject to the project record like any other defence.
//
//	[[stage]]
//	name = "shellcheck"                       # `-t` name and listed identifier
//	phase = "detectors"                       # format | detectors | runners
//	says = "ShellCheck over every shell script"
//	fix = "Quote expansions and check every status"
//	command = ["shellcheck", "{files}"]       # a whole run; {files} is one argument per file
//	files_command = ["shellcheck", "{files}"] # a subset run (-p); without it, command runs
//	files = ["**/*.sh"]                       # globs from the root choosing the files
//	shebangs = ["bash", "sh"]                 # or files whose #! line names one of these
//	exclude = ["vendor/**"]
//	dir = "web"                               # working directory, relative to the root
//	output = "findings"                       # status (the exit code) | findings (JSON on stdout)
//	rules = ["node", "scan.js", "--rules"]    # prints the rules the stage reports, as JSON
package stage

import (
	"fmt"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/LongTermSupport/go-qa-ci/internal/glob"
	"github.com/LongTermSupport/go-qa-ci/internal/pipeline"
)

// Files is the argument that expands to the chosen files.
const Files = "{files}"

// reservedPrefix is the namespace of the identifiers the toolchain itself raises.
const reservedPrefix = "gqaci."

// Output is how a stage reports.
type Output string

// The ways a stage reports.
const (
	// OutputStatus means the exit code decides and the command's own output is shown as it is.
	OutputStatus Output = "status"
	// OutputFindings means the command prints a JSON list of findings on stdout and exits 0 or 1;
	// go-qa applies the project record and fails the gate on what remains.
	OutputFindings Output = "findings"
)

// Effective is the output mode, with the default (status) filled in.
func (o Output) Effective() Output {
	if o == "" {
		return OutputStatus
	}
	return o
}

// Stage is one project stage.
type Stage struct {
	// Name is used with `-t`, and is the identifier the stage is listed and documented under.
	Name string `toml:"name"`
	// Phase is the phase the stage runs in: format, detectors or runners.
	Phase string `toml:"phase"`
	// Says is what the stage checks.
	Says string `toml:"says"`
	// Fix is how to put right what it reports.
	Fix string `toml:"fix"`
	// Command is the command for a whole run.
	Command []string `toml:"command"`
	// FilesCommand is the command for a subset run (-p), given the chosen files in the scope.
	FilesCommand []string `toml:"files_command"`
	// Files are globs, from the root, choosing the files {files} stands for.
	Files []string `toml:"files"`
	// Shebangs are interpreters: a file whose #! line names one of these is chosen too.
	Shebangs []string `toml:"shebangs"`
	// Exclude are globs, from the root, of files never chosen.
	Exclude []string `toml:"exclude"`
	// Dir is the working directory, relative to the root.
	Dir string `toml:"dir"`
	// Output is how the stage reports; empty means status.
	Output Output `toml:"output"`
	// Rules is a command printing the rules the stage reports, as a JSON list of
	// {"id", "description", "doc"?}, for the listing and the documentation check.
	Rules []string `toml:"rules"`
}

// PhaseOf is the phase the stage runs in.
func (s Stage) PhaseOf() (pipeline.Phase, error) {
	phase, err := pipeline.ParsePhase(s.Phase)
	if err != nil {
		return 0, fmt.Errorf("stage `%s`: %w", s.Name, err)
	}
	return phase, nil
}

// Problems says what is wrong with the declaration, if anything.
func (s Stage) Problems() []string {
	label := fmt.Sprintf("stage `%s`", s.Name)
	var problems []string
	problems = append(problems, s.nameProblems()...)
	if _, err := s.PhaseOf(); err != nil {
		problems = append(problems, err.Error())
	}
	if output := s.Output.Effective(); output != OutputStatus && output != OutputFindings {
		problems = append(problems, fmt.Sprintf("%s: output %q is unknown (want status or findings)", label, output))
	}
	for key, text := range map[string]string{"says": s.Says, "fix": s.Fix} {
		if strings.TrimSpace(text) == "" {
			problems = append(problems, fmt.Sprintf("%s: `%s` is empty", label, key))
		}
	}
	if s.Dir != "" && !filepath.IsLocal(s.Dir) {
		problems = append(problems, fmt.Sprintf("%s: dir %q must be a relative path inside the root", label, s.Dir))
	}
	problems = append(problems, s.commandProblems(label)...)
	for _, patterns := range [][]string{s.Files, s.Exclude} {
		for _, pattern := range patterns {
			if err := glob.Valid(pattern); err != nil {
				problems = append(problems, fmt.Sprintf("%s: %v", label, err))
			}
		}
	}
	slices.Sort(problems)
	return problems
}

// nameProblems checks the name can serve as a `-t` argument, an identifier and a page name.
func (s Stage) nameProblems() []string {
	switch {
	case strings.TrimSpace(s.Name) == "":
		return []string{"a stage has no name"}
	case strings.HasPrefix(s.Name, reservedPrefix):
		return []string{fmt.Sprintf("stage `%s`: names starting with %q belong to the toolchain's own identifiers", s.Name, reservedPrefix)}
	case strings.IndexFunc(s.Name, func(r rune) bool { return !identifierRune(r) }) >= 0:
		return []string{fmt.Sprintf("stage `%s`: the name is an identifier, so it may hold only letters, digits, `-`, `_` and `.`", s.Name)}
	default:
		return nil
	}
}

func identifierRune(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.'
}

func (s Stage) commandProblems(label string) []string {
	var problems []string
	usesFiles := false
	commands := [][]string{s.Command}
	if s.FilesCommand != nil {
		commands = append(commands, s.FilesCommand)
	}
	if s.Rules != nil {
		commands = append(commands, s.Rules)
	}
	for _, command := range commands {
		if len(command) == 0 || strings.TrimSpace(command[0]) == "" {
			problems = append(problems, fmt.Sprintf("%s: empty command", label))
		}
		for _, arg := range command {
			switch {
			case arg == Files:
				usesFiles = true
			case strings.Contains(arg, Files):
				problems = append(problems, fmt.Sprintf("%s: `%s` must be a whole argument, not part of `%s`", label, Files, arg))
				usesFiles = true
			}
		}
	}
	if (usesFiles || s.FilesCommand != nil) && !s.SelectsFiles() {
		problems = append(problems, fmt.Sprintf("%s: it selects no files; give `files` globs or `shebangs`", label))
	}
	return problems
}

// SelectsFiles reports whether the stage chooses files.
func (s Stage) SelectsFiles() bool {
	return len(s.Files) > 0 || len(s.Shebangs) > 0
}

// TakesFiles reports whether the command line chosen for the run takes the chosen files.
func (s Stage) TakesFiles(scoped bool) bool {
	return slices.Contains(s.chosenCommand(scoped), Files)
}

func (s Stage) chosenCommand(scoped bool) []string {
	if scoped && s.FilesCommand != nil {
		return s.FilesCommand
	}
	return s.Command
}

// Argv is the command line for a run, with {files} expanded to files (relative to the root, and
// passed relative to the stage directory). It reports false when the command takes files and there
// are none: the stage is skipped, never run with nothing to check.
func (s Stage) Argv(scoped bool, files []string) (argv []string, ok bool) {
	for _, arg := range s.chosenCommand(scoped) {
		if arg != Files {
			argv = append(argv, arg)
			continue
		}
		if len(files) == 0 {
			return nil, false
		}
		for _, file := range files {
			argv = append(argv, s.relativeToDir(file))
		}
	}
	return argv, true
}

func (s Stage) relativeToDir(file string) string {
	if s.Dir == "" {
		return file
	}
	rel, err := filepath.Rel(filepath.FromSlash(s.Dir), filepath.FromSlash(file))
	if err != nil {
		return file
	}
	return filepath.ToSlash(rel)
}

// Doc is the stage's documentation, for `go-qa rule-doc <name>` when the project has no page.
func (s Stage) Doc() string {
	var text strings.Builder
	fmt.Fprintf(&text, "# %s\n\nProject stage (%s phase): %s\n\n## How to fix correctly\n\n%s\n\n## What runs\n\n`%s`",
		s.Name, s.Phase, strings.TrimSpace(s.Says), strings.TrimSpace(s.Fix), strings.Join(s.Command, " "))
	if s.Dir != "" {
		fmt.Fprintf(&text, " in `%s`", path.Clean(s.Dir))
	}
	text.WriteString("\n")
	return text.String()
}
