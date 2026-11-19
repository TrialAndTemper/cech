# Approval diff

Implemented 2026-11-19; not maintained: the code and its tests are current.

A squash of fixups, or a rebase onto `main` with no conflict, keeps the approval.
When an approval is dismissed, the approval diff comment shows what changed since the approved commit.
A merge queue tests each pull request on the latest `main`, so branches need not be up to date.
See [issue 71](https://github.com/TrialAndTemper/cech/issues/71).

GitHub's own stale dismissal decides whether an approval holds; a probe showed it keeps one through a squash and a rebase that leave the pull request's diff unchanged (`research/stale-dismissal-probe.md`).
It ignores commit messages and how the commits split the work, and CECH accepts that.
CECH adds only the approval diff comment, since GitHub shows no diff across a force push that a reviewer can read.

The change also updates ADR 6 (item 5: GitHub decides whether an approval holds, and a workflow on `main` posts the diff comment; item 1: the merge queue), ADR 5 (items 2 and 3: CI on merge queue entries, and the maintainers team exempt from review), `CONTRIBUTING.md` (the rulesets, how a maintainer queues their own pull request, no rule against rewriting reviewed commits) and `AGENTS.md`, and adds `docs/reference/approval-diff.md`.

## Terms

- **Diff comment:** the approval diff comment, which shows a reviewer what changed since their approval was dismissed.
- **Dismissed approval:** an approval GitHub dismissed, by stale dismissal or by hand, whose reviewer has not approved or requested changes since, even in a review that was later dismissed.
- **Approved head:** the commit GitHub gives for a dismissed approval.
- **Replay:** CI takes the approved head's own commits and rebases them with `git rebase --autosquash` onto the current head's merge base with `main`.

## Acceptance criteria

### Whether an approval holds

1. WHEN the only pushes after an approval squash fixups, rebase onto `main` without a conflict, or both, THEN the approval holds.
   Test: `research/stale-dismissal-probe.md`; by hand, in `research/end-to-end.md` steps 3 and 7.
2. WHEN a push changes a line, adds a fixup that changes code, or resolves a conflict, THEN GitHub dismisses the approval.
   A reworded message, or the same diff split another way, keeps the approval, as GitHub decides.
   Test: `research/stale-dismissal-probe.md`

### The diff comment

3. WHEN a relay event (criterion 8) leaves a pull request with dismissed approvals, THEN the workflow posts a new diff comment for its head, with a section for each, newest first; the newest is open and the rest collapsed.
   Each section names the approved head and the current head, and holds `git range-diff` between the reviewer's replay and the current head, its fixups folded in the same way, so a fixup's change shows in the commit it fixes.
   Each section mentions the reviewer, so GitHub notifies them.
   WHEN the replay has a conflict, THEN the section compares the approved commits on their own base, with their fixups folded in if they fold cleanly, or as they are if not, and says so.
   WHEN GitHub no longer serves the approved head, THEN the section says so and gives its SHA.
   Test: `test/approval.bats` "changed line", "a fixup after the dismissal shows its change", "resolved conflict", "approved fixups that do not fold", "approved head unavailable"
4. WHEN the workflow posts a diff comment, THEN it hides its older diff comments on that pull request as outdated, and never edits or deletes one.
   WHEN a run finds no dismissed approval, such as after a new approval, a request for changes, or a squash GitHub kept the approval through, THEN it posts nothing and hides its earlier diff comments as outdated.
   WHEN a run is repeated for a head it already commented on, THEN it posts nothing again.
   Test: `test/approval.bats` "approving again posts nothing", "no dismissed approval"; by hand, in `research/end-to-end.md` steps 4 to 6.
5. WHEN the diff holds backticks, THEN its code fence is longer than any run of them.
   WHEN the diff comment is longer than GitHub allows, THEN it is cut and says so.
   A hidden marker, with the head the diff comment is for, tells the workflow's diff comments apart from other comments.
   Test: `test/approval.bats` "changed line", "backticks in the diff do not close the block, and a long comment is cut"

### How the diff comment is made

6. WHEN `scripts/approval-diff` reads the pull request's commits, THEN only git touches them, in a new repository with no hooks, no filters and no attributes, and the pull request's code never runs.
   Git comes from the official `alpine` image pinned by digest, with that Alpine release's signed packages.
   Test: `test/approval.bats` "hooks in the repository do not run", which CI also runs in the alpine image; by hand: read the image pin in `.github/workflows/approval.yml` and the `docker run` in `scripts/approval-comment`.
7. WHEN `scripts/approval-diff` ends, THEN it exits 0 having written the diff comment, or 2 if it cannot run.
   Its output gives the Git version, the base, head and merge base, and each approved head with its reviewer.
   Test: `test/approval.bats` "changed line", "approved head unavailable"

### Workflows

8. WHEN a pull request is pushed to or reopened, or a review is submitted or dismissed, THEN a relay workflow saves the pull request number.
   It runs no pull request code and has no secret.
   Test: by hand: read the workflow; `research/end-to-end.md` step 8 for a fork pull request.
9. WHEN the relay finishes, THEN a second workflow, always from `main`, runs `scripts/approval-comment`, which reads the pull request by that number and posts the diff comment.
   WHEN the pull request's head has moved since the relay ran, THEN it stops, since the newer run comments.
   WHEN the pull request's head comes from another repository than the relay ran for, THEN it stops.
   Its only write permission is `pull-requests: write`, with no secret and no `pull_request_target`.
   It posts no status, so it never blocks a merge.
   `scripts/approval-comment` refuses a missing option, a relay run id that is not digits, or an image not pinned by digest; it exits 0 when it is done or has nothing to do, and 2 when it cannot run.
   Test: `test/approval.bats` "approval-comment refuses a bad run id or an unpinned image", "approval-comment needs every option"; by hand: read `.github/workflows/approval.yml` for its permissions and `scripts/approval-comment` for the stop; `research/end-to-end.md` steps 4, 6, 8 and 9.

### Merge queue

10. WHEN CI runs on `merge_group`, THEN it checks the commits in the queue and skips `private terms`, which counts as passed.
    The pull request ran `private terms` already, and the push to `main` runs it again.
    Test: by hand: read the `private terms` job's condition in `.github/workflows/ci.yml`; `research/end-to-end.md` step 1 for a merge through the queue.
11. WHEN a member of the maintainers team queues their own pull request with no approval, THEN the queue takes it, since the team is exempt from the review ruleset.
    Test: `research/merge-queue-probe.md`
12. WHEN a reviewer withdraws an approval while the pull request waits in the queue, THEN they remove it from the queue; `CONTRIBUTING.md` says so.
    Test: by hand: read `CONTRIBUTING.md`.

## Testing before merge

GitHub runs a `workflow_run` workflow only from the default branch.
So the workflows and the scripts are first copied onto the main branch of the scratch repository `TrialAndTemper/cech-probe-merge-queue`, which has the same rulesets.
The checks by hand above run there, and their steps and results go in `research/end-to-end.md`.
The scratch repository is deleted after this change merges, so reviewers can see the runs.

## Ruleset changes by hand

After this change merges, since GitHub runs a `workflow_run` workflow only from `main`:

- Review ruleset: turn on "dismiss stale reviews" and turn off "require approval of the most recent push".
  Keep one code owner approval.
  Make the maintainers team exempt instead of letting admins bypass, so a maintainer can queue their own pull request (`research/merge-queue-probe.md`).
- Gate ruleset: turn on the merge queue with rebase, and turn off "require branches to be up to date".

## Out of scope

- Checking commit messages, or how the commits split the work, after an approval: GitHub keeps the approval, and CECH accepts that.
- A diff comment for approvals GitHub keeps: nothing a reviewer must look at again changed.
- A third-party Action for the diff comment: the available ones compare push with push, not the approved head with the current head.
- Pull requests that target a branch other than `main`: the rulesets guard only `main`.
