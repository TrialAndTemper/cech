# ADR link gate

Closes #84: `scripts/check-adr`, run as `mise run lint:adr` and by the pre-commit hook, also holds ADRs to the links rule of ADR 1 item 6.

## Acceptance criteria

1. A link `[ADR N](NNNN-name.md)` in an ADR whose target has a higher number than the ADR itself is a finding that names the file, the line and both numbers.
   A link to an earlier ADR, or to the ADR itself, passes.
   A link with a `#` anchor is read by its file name.
   Test: `test/adr.bats` "a link to a later ADR is a finding"
2. A link in the `- Status:` line of an ADR passes, so `superseded by ADR N` may point forward.
   The exception covers the whole status line, not only the words `superseded by`.
   Test: `test/adr.bats` "a link in a superseded by status may point to a later ADR"
3. A link whose text names a different ADR from the file it links to, such as `[ADR 14](0026-the-release-check.md)`, is a finding.
   Test: `test/adr.bats` "a link whose text names another ADR than its file is a finding"
4. A link inside a fenced code block is not read, with the fences the word count reads.
   A link to a file that is not `NNNN-*.md` is not read.
   Test: `test/adr.bats` "a link in a code block or to another file is not read"
5. A line with more than one link reports each one that breaks the rule.
   Test: `test/adr.bats` "each link on a line is checked"
6. ADR 1 item 4 says a gate checks the links, and ADR 5 item 3 no longer links to ADR 6.
   ADR 1 item 5 loses the words "whatever the change,", which keeps ADR 1 under the 800-word limit and its meaning: an accepted ADR is edited in place until the first plugin merges.
   Test: `mise run lint:adr` on the tree
7. CONTRIBUTING.md names the links in what `lint` checks and in the ADR rule.
   Test: by hand: read both lines.

## Out of scope

- Links in `docs/adr/README.md`, which lists every ADR and is not an ADR.
- Links to ADRs from outside `docs/adr/`.
- Reference-style links, and links written with a `./` or `../adr/` prefix, which the ADRs do not use: ADR 1 item 6 sets the form `[ADR N](NNNN-title.md)`.
