# The approval diff comment

When GitHub dismisses an approval, the approval diff comment on the pull request shows what changed between the approved commits and the current head ([ADR 6](../adr/0006-change-record.md) item 5).
GitHub decides whether an approval holds; the diff comment never blocks a merge.
How to work with it is in [CONTRIBUTING.md](../../CONTRIBUTING.md#pull-requests).

## When an approval holds

The review ruleset dismisses stale approvals.
GitHub keeps an approval through a squash of fixups and a rebase onto `main` with no conflict, and dismisses it for a changed line, a fixup that changes code, or a resolved conflict.
It keeps it, too, for a reworded message or the same diff split into other commits.
GitHub does not publish these rules; they were probed on 2026-11-18.

## The diff comment

A dismissed approval is one whose reviewer has not approved or requested changes since; a later approval or request for changes ends it even if that review was dismissed too, and an ordinary review comment does not end it.
The approved head is the commit GitHub gives for it.

After each workflow run that leaves a pull request with dismissed approvals, the workflow posts a new diff comment for its head, unless it already has one.
It has a section for each, newest first; the newest is open, and the rest are collapsed.
Each section mentions the reviewer, so GitHub notifies them, and names the approved head and the current head.
It holds `git range-diff` between the approved head's own commits, replayed with `git rebase --autosquash` onto the current head's merge base with `main`, and the current head's commits with their fixups folded in the same way.
So a fixup's change shows in the commit it fixes, as it will merge; if the head's fixups do not fold cleanly, the section uses the head as it is.
The patch lines are moved left, so GitHub colours them: red lines are from the approved commits, green lines from the current head, and each commit's header is purple.
A commit with no match on the other side, such as one added or dropped after the approval, shows as its header only; find it by its subject in the pull request's commits to see its change.
When the replay has a conflict, the section compares the approved commits on their own base, with their fixups folded in if they fold cleanly, or as they are if not, and says so.
When GitHub no longer serves the approved head, the section says so and gives its SHA.

Each diff comment is a snapshot for one head and is never edited.
A newer diff comment hides the older ones as outdated, and anyone can expand them.
A run with no dismissed approval posts nothing and hides the earlier diff comments as outdated; they stay as history.
A run for a head it already commented on posts nothing.

## How it runs

- `.github/workflows/approval-relay.yml` runs when a pull request is pushed to or reopened, and when a review is submitted or dismissed, and saves only the pull request's number.
  Opening a pull request, or editing its title, its description or a review, changes no approval, so it does not run then; a pull request retargeted onto `main` gets its diff comment at its next push or review.
- `.github/workflows/approval.yml` runs after it, from `main`, and calls `scripts/approval-comment` with the workflow's own token.
- `scripts/approval-comment` reads that pull request, and stops if its head has moved or comes from another repository than the relay's, or if it is closed or does not target `main`.
  It reads the reviews, and which dismissed reviews were approvals, from GitHub's API.
  It runs `scripts/approval-diff` in the official `alpine` image, pinned by digest, with git from Alpine's signed packages.
  Only git reads the pull request's commits, in a new repository with no hooks, filters or attributes.
  It posts and hides diff comments, and posts no status.
  It exits 0 when it is done or has nothing to do, and 2 when it cannot run.

`scripts/approval-diff` exits 0 when it has written the diff comment, or found no dismissed approval, and 2 when it cannot run.
It prints the Git version, the base, head and merge base, and each dismissed approval's head and reviewer.
`mise run test` runs its tests in `test/approval.bats`, and CI also runs them in the same `alpine` image, under BusyBox.
The suite checks only the options of `scripts/approval-comment`, since the rest of the script calls GitHub's API.
GitHub runs `approval.yml` only from `main`, so a change to it or to either script is checked by hand: copy them onto the `main` of a scratch repository that has the same rulesets.
There, push a change that dismisses an approval, add a fixup, approve again, push from a fork, and push twice a few seconds apart, and check the diff comment each time.
