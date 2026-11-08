# ADR numbering and index gate

Implemented 2026-10-29; not maintained: the code and its tests are current.

`scripts/check-adr`, run as `mise run lint:adr` and by the pre-commit hook, also holds ADRs to ADR 1 items 4 and 7.

## Rules

1. The tracked `docs/adr/NNNN-*.md` files are numbered from 1, with no gap and no number used twice.
2. Each starts with the title line `# N. Title`, where `N` is the number in its file name and the title is not blank.
3. `docs/adr/README.md` exists and lists each ADR once, in order, as `N. [Title](NNNN-name.md)`, with that ADR's title and file name, and has no other numbered list item, indented or not.
4. Each finding names the file, and the line in the index where there is one; findings exit 1.
