# gqaci.docs.orphan

## What it flags

A page in the project documentation directory (`docs_dir`, `docs/defences` by default) that
documents no identifier the project can see: its file name, compared without case and with `-` read
as `_`, matches no built-in identifier and no rule in the project record. `README.md` is exempt.

## Why

A page outlives the rule it described when the rule is removed or renamed. A reader who finds it
then follows a defence that no longer runs, and the directory stops being a true list of what the
gate enforces.

## How to fix correctly

Delete the page if its defence is gone. If the defence was renamed, rename the page to the
identifier it now prints. If the defence should still run, restore it so that it is listed again.

## Suppression

Not suppressible. Delete, rename or restore.
