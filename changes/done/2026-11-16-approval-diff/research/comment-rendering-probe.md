# Comment rendering probe

Run on 2026-11-19 with GitHub's Markdown API in `gfm` mode and a comment on a pull request in the scratch repository `TrialAndTemper/cech-probe-merge-queue`.

## Question

Can a pull request comment show a diff in colour, or graphically, rather than as plain text?

## Steps and results

The same change, a changed line, an added line and a reworded message, shown four ways:

| Form | Rendering |
| --- | --- |
| `git range-diff` in a plain code block | Plain text |
| `git range-diff` in a `diff` code block | Plain text: each changed line starts with spaces, so no line counts as removed or added |
| `git range-diff` in a `diff` code block, with the four spaces before each patch line removed and each commit's header line wrapped in `@@ … @@` | Lines from the approved commits in red, lines from the current head in green, commit headers in purple, a reworded message line in red and green |
| `git diff` between the two trees in a `diff` code block | Removed lines in red, added lines in green; no commit messages and no commit boundaries |

## Findings

- A comment cannot show GitHub's side-by-side diff view; a `diff` code block, coloured by each line's first character, is the closest it gets.
- With its patch lines moved left, `git range-diff` renders in colour and still shows the commit messages and which commit each change is in, so the comment uses that form.
