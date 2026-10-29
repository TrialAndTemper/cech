# Working material for changes

`.ai/` holds material written mostly by and for agents, one folder per change: specs, briefs, research and spike results.
People review it with the change; the documents written for people are in `docs/` ([ADR 1](../docs/adr/0001-record-architecture-decisions.md) item 8).

## Layout

```text
.ai/changes/YYYY-MM-DD-short-title/        a change in progress
  spec.md                                  what the change must do, or specs/ for several
  brief.md, research/, spikes/             anything else the change needs
.ai/changes/done/YYYY-MM-DD-short-title/   a finished change, kept as history
```

The date is the day the folder is created; it stays when the folder moves.

Only `spec.md` or `specs/` has a fixed name; add other files and subfolders as the change needs them.

## When a change needs a folder

A change needs a spec when it adds or alters detail that users, scripts or later changes rely on, such as a key, a flag, an exit status or a message.
A bug fix that restores what a spec or test already says needs none.
Add a brief, research or spike results when they help review, or help the next person or session pick the change up.
Any other change needs no folder.

## From start to done

- Write the spec before or with the code, and review it like code; the decision and the reasons behind it stay in the ADR the change follows, if any.
- Change the folder with the code until tests cover every rule in its spec.
- Then the pull request that moves it names the test that covers each rule.
  It moves the folder with `git mv` to `done/`, updates links to it in the tree, and adds the line `Implemented YYYY-MM-DD; not maintained: the code and its tests are current.` under the spec's title, dated the day that pull request is written.
- A folder without a spec moves to `done/` with the pull request that finishes its change.
- Never edit a folder in `done/` apart from typos and links; a later change starts a new folder.

## Rules for everything here

- Never commit notes that quote private or unpublished sources; rewrite their findings in your own words ([ADR 4](../docs/adr/0004-vendor-neutrality.md)).
- An ADR never links here: it states the findings it rests on in its own words, since folders here move.
- A commit message links here only at a fixed commit, for the same reason.
