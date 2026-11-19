# Stale dismissal probe

Run on 2026-11-18 in the scratch public repository `TrialAndTemper/cech-probe-merge-queue`, with gh 2.86.0 and GitHub's REST API.

## Question

With the review ruleset's "dismiss stale approvals" on, which pushes after an approval does GitHub dismiss it for?

## Set-up

- The rulesets of [the merge queue probe](merge-queue-probe.md), with `dismiss_stale_reviews_on_push` turned on in the review ruleset for the probe and off again after it.
- A pull request opened by a collaborator, from a branch in the repository, which the maintainer approved and pushed to.
- After each push, the maintainer read the reviews and the review decision 25 seconds later.

## Steps and results

| Push after the approval | Approval | Review's commit |
| --- | --- | --- |
| A squash of a fixup, on the same base | Kept | Moved to the new head |
| A rebase onto a newer `main` that touches no file of the branch | Kept | Moved |
| A reworded message, with the same tree | Kept | Moved |
| Two commits folded into one, with the same final tree | Kept | Moved |
| A changed line, as a force push | Dismissed | Left on the old head |
| A fixup that changes a line, as a plain push | Dismissed | Left on the old head |
| A new approval of the head with that fixup, then a squash | Kept | Moved |
| A rebase onto a `main` that conflicts, with the conflict resolved | Dismissed | Left on the old head |

## Findings

- GitHub keeps an approval through a squash and a rebase that leave the pull request's diff unchanged, and dismisses it for a changed line, a fixup that changes code and a resolved conflict.
- It ignores commit messages and how the commits split the work: a reworded message or a different split with the same diff keeps the approval.
- Whenever it keeps an approval after a force push, it moves the review's commit to the new head, so the API no longer gives the commit the reviewer approved.
- GitHub does not publish these rules, so they can change without notice.
