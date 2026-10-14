# Project stages and resolvers

A project's own defences and runners belong to its Toolchain as much as the bundled ones
(TOOLING-SPEC 5.3): a command no built-in tool covers (another language's formatter or linter, a lint
library's own suite, a check that a generated file is fresh) is declared as a `[[stage]]` in
`qa/go-qa.toml` and runs in the phase it names. A stage is listed by `go-qa tools`, selected with
`-t <name>`, documented under its name, and, when it reports findings, subject to the project
record like any other defence.

```toml
[[stage]]
name = "shellcheck"                       # `-t` name and listed identifier
phase = "detectors"                       # format | detectors | runners
says = "ShellCheck over every shell script"
fix = "Quote expansions and check every status"
command = ["shellcheck", "{files}"]       # a whole run; {files} is one argument per file
files_command = ["shellcheck", "{files}"] # a subset run (-p); without it, command runs
files = ["**/*.sh"]                       # globs from the root choosing the files
shebangs = ["bash", "sh"]                 # or files whose #! line names one of these
exclude = ["vendor/**"]
dir = "web"                               # working directory, relative to the root
output = "findings"                       # status (the exit code) | findings (JSON on stdout)
rules = ["node", "scan.js", "--rules"]    # prints the rules the stage reports, as JSON

[[resolver]]
matches = "SC[0-9]*"                      # a glob over identifiers
text = "See the ShellCheck wiki page for {id}."
```

`name`, `phase`, `says`, `fix` and `command` are required. Unknown keys are rejected. The name is an
identifier (letters, digits, `-`, `_`, `.`), unique, not the name of a built-in tool, and outside the
`gqaci.` namespace that belongs to the toolchain's own identifiers.

## Phases and order

Within a phase the built-in tools run first, in catalogue order, then the stages in declared order.
A failed format or detector stage stops the run before the first runner, like any other
(TOOLING-SPEC 4.5). A stage always runs on a default run; there is no `[tools]` switch for it, and
`-t <name>` runs only it.

## Files

`{files}` must be a whole argument; it expands to one argument per chosen file, relative to `dir`
when one is set. Files are chosen from the project root by the `files` globs (`**` matches any number
of directories) or by `shebangs` (the interpreter of the `#!` line, looking through `env`), minus
`exclude`. Inside a git work tree the candidates are the tracked and untracked files that
`.gitignore` does not ignore; elsewhere every file outside `.git`. Symbolic links are never chosen.
A stage whose command takes `{files}` and finds none is skipped, and the run summary says so; it
is never run with nothing to check. A stage that selects files is declared with `files` or
`shebangs`, or the configuration is rejected.

With `-p <path>` the run is limited to that scope: the chosen files are those inside it, and
`files_command` runs instead of `command` when the stage has one. A stage with no `files_command`
runs whole.

## Output

- `status` (default): the exit code decides, and what the command printed is shown.
- `findings`: the command prints a JSON list on stdout and exits 0 or 1:
  `[{"file": "src/a.ts", "line": 4, "rule": "web-rule", "message": "...", "note": "..."}]`
  (`column` and `note` optional, `file` relative to `dir`, no other keys). go-qa applies the project
  record (`covers = "findings"` exceptions for the stage's rule ids and paths) and prints what remains
  compiler-style. A stage that exits with another code, prints text that is not that list, or exits 1
  while reporting nothing did not do its job and fails; none of these is a pass. On a run over the
  whole project, a findings exception for a rule the stage's `rules` command prints and that licensed
  nothing is `gqaci.record.stale`.

## Rules, documentation, resolvers

`rules` is a command printing `[{"id": "...", "description": "...", "doc": "..."}]` (`doc` optional)
for the rules the stage reports. The command failing, or printing something else, is an error, never
an empty list.

An identifier resolves, in order, to a project page in `docs_dir`, the stage of that name (its `says`,
`fix` and command), the `doc` a stage's rules command ships for that rule, a bundled page, the
documentation of a registered analyzer, and the first `[[resolver]]` whose glob matches, with `{id}`
replaced. `go-qa docs-check` and the `docs` detector require each stage name and each rule a stage
prints to resolve (`gqaci.docs.dangling`), and a page for either is not an orphan.

## Recorded gaps (TOOLING-SPEC 9.2)

- The defence listing (`go-qa rules`, M11) does not list stages or their rules yet; the docs
  detector enumerates them itself. The listing, the summary and the `summary_rules` switch arrive
  with M11.
- A findings exception whose rule no defence reports is not yet rejected as invalid for stage rules;
  only the unused-exception check above applies.
- Stage commands run with the environment go-qa was started in; a stage's `dir` selects the working
  directory only, and a per-directory toolchain is the stage's own business.
