# End-to-end test

Run on 2026-11-19 in the scratch public repository `TrialAndTemper/cech-probe-merge-queue`, whose `main` held this change's script and workflows.
Its rulesets were those in the spec's "Ruleset changes by hand": the review ruleset dismissed stale approvals, required one code owner approval and exempted the maintainers team, and the gate ruleset required the merge queue.
Steps 4 to 6 and 8 ran again once the workflow's steps had moved into `scripts/approval-comment`, with the same results.

## Steps and results

1. The maintainer opened a pull request with no approval and queued it with `enqueuePullRequest`.
   The queue ran CI on `merge_group` and merged it.
2. On a pull request by a collaborator, the maintainer approved.
   The run found no dismissed approval and posted nothing.
3. A rebase onto a newer `main` with no conflict kept the approval, and nothing was posted.
4. A force push that changed a line made GitHub dismiss the approval.
   The push and the dismissal each started a run: one posted the comment, with the changed line in red and green, and the other found a comment for that head and posted nothing.
5. A fixup with a further change, before a new approval, posted a new comment.
   It showed the change inside the commit the fixup fixes, and the earlier comment was hidden as outdated.
6. The maintainer approved again: nothing was posted, and the last comment was hidden as outdated.
7. A squash of the fixup and a rebase onto a newer `main` kept that approval, and nothing was posted.
8. On a pull request from a fork, the relay ran with the fork's read-only token.
   A changed push made GitHub dismiss a collaborator's approval, and the second workflow posted the comment on that pull request.
9. Two pushes a few seconds apart, with an approval still dismissed, gave one comment, for the second head.
   GitHub replaced the relay run for the first push before it finished, so no run commented on a head that had moved.
