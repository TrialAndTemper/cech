# Git hooks

Implemented 2026-10-29; not maintained: the code and its tests are current.

The optional hooks of ADR 4 item 4, in `scripts/hooks/`, which `git config core.hooksPath scripts/hooks` turns on.

## Rules

1. `pre-commit` runs `mise run pre-commit`, which checks what is staged, not the files on disk:
   - the term check on the staged changes;
   - in a checkout of the index made by `scripts/staged-tree`, the shell, Go, workflow and Markdown lint checks, and the shell, Go and Markdown format checks.
2. `commit-msg` runs `mise run commit-msg` on the message file: the term check and the commit message gate, which checks the sign-off against the author that git will record, as CI does.

Neither hook runs the tests or the checks that need the network, such as the link check.
