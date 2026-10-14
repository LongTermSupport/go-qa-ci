# The ast-grep detector

The `ast-grep` tool runs the project's syntactic rules, the cheap single-file Defences where syntax
provably suffices (DETECTOR-SPEC 4.1, bespoke rules), and holds those rules to the same standard as
every other Defence: each has an identifier, and each has a red proof and a green proof.

## What it does

1. Reads `sgconfig.yml` (`ruleDirs`, `testConfigs`). Every directory it names must lie inside the
   project root.
2. Judges the rules without running anything:
   - `gqaci.rules.unnamed`: a rule file holding a rule with no `id`.
   - `gqaci.rules.untested`: an error-severity rule with no test case holding both `valid` and
     `invalid` code in a `testConfigs` directory. A rule below error severity does not block, so it
     needs no proof.
3. Runs `ast-grep scan --json=compact` for the project rules, then once more with the bundled pack
   (below), limited to the `-p` paths of the run. Only error-severity matches block; a match the
   project record licenses (`covers = "findings"`, the rule as the identifier) is dropped.
4. Runs `ast-grep test --skip-snapshot-tests` when `sgconfig.yml` declares `testConfigs`; a
   failure fails the step and prints ast-grep's output.
5. On a run over the whole project, fails a findings exception for a rule this tool hosts that
   licensed nothing (`gqaci.record.stale`). A run limited to some paths cannot judge staleness.

Findings print compiler-style with the rule identifier exactly as ast-grep reports it.

## Without the binary

The executable is `ast-grep`, never `sg`: on Linux `sg` is another program. When it is not on
`PATH` the step fails with `ast-grep is not installed`, after the rules themselves have passed
the checks in item 2; a rule with no id or no proof fails whether the binary is installed or not.
A missing binary is an error like any other missing tool, never a skip, so no runner follows it.
ast-grep is pinned in `tools.lock` and installed by `scripts/install-tools.sh`.

With no `sgconfig.yml` and an empty or switched-off pack there is nothing to scan, and the step is
skipped saying so.

## The bundled pack

`internal/astgrep/pack/gqaci.ast.<name>.yml` files are embedded in the binary. The pack runs from a
private temporary file, never from the project tree, and applies `scan_exclude` as `--globs`.
`[ast_grep] bundled = false` switches it off.

Every bundled rule must: live in the `gqaci.ast.` namespace, be in a file named after its id,
target Go, have error severity, carry a note pointing at its page (`go-qa rule-doc <id>`), and have
red-proof fixtures in `tests/astgrep`. These are checked when the pack loads, so a malformed rule
fails every run rather than one test.

**The pack is empty.** gosec already covers disabled TLS verification (G402) and shell execution
(G204) in the strict baseline, so duplicating them would only add a second voice. A rule is added
only where syntax suffices and no enabled linter covers the ground; the machinery, its integrity
checks and its fixtures are in place for the first one.

## Suppression

An `// ast-grep-ignore: <rule>` comment is a suppression route: the `suppression` tool requires the
project record to license it, and a bare one (no rule) is `gqaci.suppression.bare`
(docs/suppression.md).

## Recorded gaps

- The tests that need the real binary (`tests/astgrep`: each rule fires on a violating Go snippet
  and stays quiet on a clean one, the project's rule tests pass, an ignore comment silences a rule,
  a project rule and a bundled rule are reported in one run) are skipped when it is not on `PATH`.
  CI installs the pinned ast-grep (`tools.lock`) before `go test`, so CI runs them; elsewhere they
  run wherever the binary is installed (verified with ast-grep 0.50.0).
  The detector's own logic is tested everywhere with a stand-in executable
  (`internal/astgreptest`).
- A project rule's identifier does not yet resolve to a page through `go-qa rule-doc`: the defence
  listing (`docs/enumeration.md`) lists project rules, but the docs detector does not take its identifiers
  from it yet and treats only built-ins and rules named in the project record as identifiers the
  project can see.
- `ast-grep` is not invoked with a pinned version check; the version in use is whatever is on
  `PATH`.

## Writing Go call patterns

A bare pattern such as `fmt.Println($$$ARGS)` never matches Go: tree-sitter-go reads it as a type
conversion, not a call, so the rule silently never fires and `ast-grep test` then fails the
project. Give the pattern a context and select the call:

```yaml
rule:
  pattern:
    context: "func t() { fmt.Println($$$ARGS) }"
    selector: call_expression
```
