# Merge queue probe

Run on 2026-11-17 in a scratch public repository, `TrialAndTemper/cech-probe-merge-queue`, with gh 2.86.0 and GitHub's REST and GraphQL APIs.

## Question

Can a maintainer merge their own pull request, which has no approval, once a merge queue is required?
Does the queue then need the review ruleset's required status on its own commit?

## Set-up

- Gate ruleset `main`, no bypass: no deletion, no force push, linear history, pull request with no approval and rebase only, required check `ci` from GitHub Actions, merge queue with rebase.
- Review ruleset `main review`: one code owner approval, and a required commit status from GitHub Actions; the maintainers team is exempt.
- `CODEOWNERS` names the maintainers team; CI runs `ci` on `pull_request` and `merge_group`.
- A maintainer opened a pull request and did not approve it; nothing posted the required status on its head.

## Steps and results

1. `gh pr merge --rebase` did not queue it: gh fell back to auto-merge, which the repository has off.
2. The GraphQL mutation `enqueuePullRequest` queued it, although the review state was "review required" and the required status was missing on the head.
3. The queue built its commit and `ci` passed on it, but the entry stayed "awaiting checks": the queue wanted the required status on its own commit, exempt or not.
4. A push to the queued branch was refused until the pull request left the queue.
5. With a `merge_group` workflow that posts the required status as passed using the workflow's token, the queue merged the pull request.
   The status counted as coming from GitHub Actions.
6. `main` got the pull request's commits on top, with the committer changed to the maintainer's login.
7. A collaborator with write access who is not exempt opened a pull request with no approval and, once `ci` passed, clicked "Merge when ready" in the web page.
   GitHub did not queue it: it stored a request to merge once the rules are met, the pull request stayed "review required", and the queue stayed empty.
   Auto-merge is off in the repository, so this stored request is part of the merge queue.
   GitHub does not let an author approve their own pull request.
8. A second collaborator who is not exempt opened another such pull request.
   The web page showed no option to queue it, only the pending approval.
   `enqueuePullRequest` failed with the error `UNPROCESSABLE`, giving both missing parts: the code owner review from the maintainers team and the required status.
   The queue stayed empty.

## Findings

- Exempting the maintainers team from the review ruleset lets a maintainer queue their own pull request.
- The queue requires every required status, including the review ruleset's, on the queue's commit.
- A status posted with the workflow token meets a requirement for that status from GitHub Actions.
- A collaborator who is not exempt cannot queue an unapproved pull request, from the web page or the API: the queue checks the review ruleset, and its required status on the head, before it takes the pull request.
