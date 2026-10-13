# go-qa-ci

A Defence Before Fix (DBF) Toolchain for Go modules. Work in progress: no conformance to the DBF
Tooling Specification is claimed. No declaration or gap list exists yet; one is added when the Toolchain has something to declare.

## Status

Only the command skeleton exists (`go-qa --version`). See `docs/PUBLICATION-CHECKLIST.md` before
this repository is made public.

## Public safety

`scripts/public-safety-scan.sh` fails on any term from a denylist kept outside the repository, and
`scripts/secret-scan.sh` runs gitleaks. Enable the hooks with `git config core.hooksPath .githooks`.
The CI workflow requires the `GQACI_DENYLIST` repository secret and fails without it.
