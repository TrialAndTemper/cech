# Working material for changes

`changes/` holds the working material of each change, written by people and agents alike: specs, test plans, briefs, research and spike results ([ADR 24](../docs/adr/0024-where-each-kind-of-text-lives.md) item 3).
It is not a changelog; release notes are written from the commits.
What CECH does now is in [`docs/reference/`](../docs/reference/README.md) and the code, and why is in the [ADRs](../docs/adr/README.md).

## Layout

```text
changes/YYYY-MM-DD-short-title/        a change in progress
  spec.md                              its rules and test plan, or specs/ for several
  brief.md, research/, spikes/         anything else the change needs
changes/done/YYYY-MM-DD-short-title/   a finished change, kept as history
```

The date is the day the folder is created; it stays when the folder moves.

Only `spec.md` or `specs/` has a fixed name; add other files and subfolders as the change needs them.

## When a change needs a folder

A change needs a folder when it adds or alters a rule that users, scripts or later changes rely on, such as a key, a flag, an exit status or a message.

- If a page in `docs/reference/` describes that behaviour, the change edits the page, and the edit is its spec.
  The folder's spec then lists the criteria with their tests and points to the page for the rule itself.
- Any other rule, such as how a gate behaves or what a command not yet built must do, is written in full in the folder's spec.

A bug fix that restores what a spec, a reference page or a test already says, and any change not listed above, needs no folder.
Add a brief, research or spike results when they help review, or help the next person or session pick the change up.

## A spec

Keep it short: someone writing by hand should not face a form.

```markdown
# <Change title>

<Goal: one to three sentences, or a link to the issue.>

## Acceptance criteria

1. <A rule a test can check.>
   Test: `test/<suite>.bats` "<case name>"
2. <See docs/reference/<page>.md#<section>, edited with the code, plus anything the page does not say.>
   Test: by hand: <how>

## Out of scope

- <What this change leaves alone on purpose.>

## Open questions

- <Each is answered before the folder moves to done/.>
```

- The goal, the criteria and their `Test:` lines are required; the last two sections are optional.
  Until the test exists, a criterion's line is `Test: to be written`.
- Number the criteria, so commits and test comments can cite them.
- Group the criteria under subheadings, or add a section such as an example, where that helps.
- The reasons for a decision belong in its ADR or commit message; the steps and raw results behind its findings go in research beside the spec.
- A spec in an open change that predates this template may keep its form, and link and typo fixes do not change that; specs in `done/` keep theirs.
- A task list may sit beside the spec as its own file while the change is in progress.

## From start to done

- Write the spec before or with the code, and review it like code.
- Change the folder with the code until a test, or a recorded check by hand that says how, covers every criterion.
- The pull request that finishes the change fills in the last `Test:` lines, moves the folder with `git mv` to `done/`, updates links to it in the tree, and adds the line `Implemented YYYY-MM-DD; not maintained: the code and its tests are current.` under the spec's title, dated the day that pull request is written.
- A folder without a spec moves to `done/` with the pull request that finishes its change, with no `Implemented` line.
- Never edit a folder in `done/` apart from typos and links; a later change starts a new folder.

## Rules for everything here

- Never commit notes that quote private or unpublished sources; rewrite their findings in your own words ([ADR 3](../docs/adr/0003-vendor-neutrality.md)).
- An ADR or a reference page never links here: it states what it rests on in its own words, since folders here move.
- A commit message links here only at a fixed commit, for the same reason.
