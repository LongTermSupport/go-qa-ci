# Bundled ast-grep pack

Each `gqaci.ast.<name>.yml` file here is one bundled rule, embedded into the binary and run by the
`ast-grep` detector unless `[ast_grep] bundled = false`.

The pack is empty by design. A rule belongs here only where syntax provably suffices and no linter
that the strict baseline enables covers the same ground: gosec already covers disabled TLS
verification (G402) and shell execution (G204), so duplicating them would only add a second voice.
A rule added here needs a page under `ruledocs/rules/`, a note of the form
`go-qa rule-doc <id>`, and red-proof fixtures in `tests/astgrep`.
