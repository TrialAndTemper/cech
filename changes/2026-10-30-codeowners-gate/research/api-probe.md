# Code owners errors API probe

How GitHub's `GET /repos/{owner}/{repo}/codeowners/errors?ref=<sha>` answers, checked by hand with curl and `gh api`.

## This repository, 2026-10-30

- A commit without a code owners file: 404, the same answer as for a repository the token cannot read.
- A commit with a clean file: 200 with `{"errors":[]}`, also for a pull request's merge commit.
- An unknown commit: 500.
- A bad token: 401.
- Without a token, the public repository answers too.

## This repository, 2026-11-20

The gate, run by hand with a personal token against `main` at `c16e77e`, printed `codeowners: ok (.github/CODEOWNERS at c16e77e…)`.

## A scratch repository, 2026-11-20

A public scratch repository in the same organisation as CECH had a `main` whose code owners file named one owner with write access, a ruleset with a merge queue that requires the gate's check, and the gate in a workflow on `pull_request` and `merge_group`.
A pull request added four lines: a user that does not exist, a real user without write access, a team that does not exist, and a pattern with an unescaped `[`.

- For `main`, GitHub answered 200 with no errors; for the pull request's head, 200 with four errors, one per line.
- Each error has `line`, `column`, `source`, `kind` (`Unknown owner` or `Invalid pattern`), `suggestion`, `message` and `path`.
  The `message` runs over several lines: a sentence such as `Unknown owner on line 2: make sure @… exists and has write access to the repository`, then the line quoted with a caret under the fault.
  So the gate prints only the message's first line.
- A user without write access gets the same `Unknown owner` error as one that does not exist.
- On `pull_request`, with `GITHUB_SHA` the merge commit of `refs/pull/1/merge`, the gate reported the four errors and exited 1.
- In the merge queue, with `GITHUB_SHA` the queue's commit on `gh-readonly-queue/main/pr-1-…`, it reported the same four errors and exited 1, and the queue dropped the pull request.
  So GitHub reads `ref` for both commits rather than falling back to the default branch, whose file had no errors.
- The queue takes a pull request only once its required checks pass, so on `pull_request` the probe's workflow ignored the gate's exit status, and only on `merge_group` did a failure fail the job.

The probe's workflow used the runner's own tools, not the pins in `mise.toml`: curl 8.5.0 and jq 1.7 on `ubuntu-24.04`.
