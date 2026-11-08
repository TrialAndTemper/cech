# ADR length gate

Implemented 2026-10-29; not maintained: the code and its tests are current.

`scripts/check-adr`, run as `mise run lint:adr` and by the pre-commit hook, holds ADRs to the limit of ADR 1 item 6.

## Rules

1. It checks every tracked `docs/adr/NNNN-*.md`; with none, it cannot run and exits 2.
2. An ADR fails with 800 or more words of prose outside fenced code blocks.
   A word is a run of characters between spaces or tabs.
3. Indents are counted in columns, a tab moving to the next multiple of 4, and inside a list item from the item's content column, which is one column after the marker when five or more blank columns follow it.
   A list item starts with a marker followed by a space or tab, except on a thematic break or in indented code.
   After a paragraph line, a numbered marker other than `1.` starts an item only inside a list.
   A fence opens on a line whose first non-blank text is three or more backticks or tildes, indented less than four; a backtick run followed by a backtick on the same line opens none.
4. A fence closes only on a line of the same character, at least as long, with nothing but blanks after it, and indented less than four columns into its container: the list item's text, or column 0.
5. A fence that never closes is a finding, named by the line it opens on, even one inside a list item, which CommonMark closes with the item.
   A fence after a list marker or `>` on the same line, as in a block quote, is read as prose.
6. The pre-commit hook checks the staged ADRs, not the files on disk.
7. Exit status: 0 clean, 1 findings, 2 the check could not run.
