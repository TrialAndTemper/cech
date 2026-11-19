# 6. Change record

- Status: accepted
- Date: 2026-10-17
- Deciders: Michał Bożek

## Context

People reading the history later need to know what changed and why.
Pull request text lives on GitHub, not in git: forks, mirrors and offline clones do not have it.
Links to pages that need a login or expire break the same way.
Squash merges fold a pull request into one commit and lose its steps; merge commits make the history hard to read and bisect.
Release notes need commits they can sort by kind.

Folding review fixes in rewrites the commits a reviewer approved.
GitHub shows no readable diff across a force push.
Probed on GitHub on 2026-11-18: with stale approvals dismissed, GitHub keeps an approval through a squash and a rebase that leave the diff unchanged, and dismisses it for any other change; it does not publish these rules.
Probed on 2026-11-17: a merge queue takes an unapproved pull request only from a team exempt from review, not from a ruleset bypass.

## Decision

1. **Rebase merge only.**
   `main` takes only rebase merges, so its history stays linear, and refuses force pushes; the host enforces both.
   A merge queue runs the gates on the latest `main` before it merges a pull request, so a branch needs no rebase just because `main` moved.
2. **Commits are the record.**
   Anything that must last (the reason for a change, evidence, rejected options) goes in a commit message or an ADR.
   The pull request description summarises and links; it is not the only place for any of that.
   A link in a commit message points only to a public page that lasts.
   Each commit is one logical change, and review fixes are folded into the commits they fix before merge.
3. **Messages follow [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/),** with a fixed set of types, so release notes can sort commits by kind.
   The scope is for readers: a plugin may share a name with an area, so tools that need a commit's plugins read the paths it changes.
4. **Every commit is signed off by its author** ([ADR 3](0003-licence-and-contributions.md)), since the DCO is the author's statement.
   A gate checks each commit's header and sign-off, and that no fixup or merge commit reaches `main`; the rest of the message rules are for reviewers.
5. **GitHub decides whether an approval holds, and the approval diff comment shows what changed.**
   The review ruleset dismisses stale approvals, so folding fixups in or rebasing without a conflict keeps an approval, and any other change to the diff needs a new one.
   A reworded message, or the same diff split another way, keeps the approval, as GitHub decides.
   When GitHub dismisses an approval, the approval diff comment shows the reviewer what changed since, as `git range-diff` of the approved commits against the current head.
   The diff comment never blocks a merge.
   A workflow that runs from `main` posts it and trusts nothing from the pull request's own run but its number, so a fork cannot change what it shows ([ADR 5](0005-gates-and-tool-pins.md) item 6).
   The script and its tests run in a digest-pinned image, with git and bats from that image's signed packages.
   This is an exception to [ADR 5](0005-gates-and-tool-pins.md) item 8's mise pins: the container holds nothing but git to read the pull request's commits.

## Consequences

- The reason for each change is in its commit.
- Authors spend some time shaping commits before merge.
- Pull request reviewers read commits, not only the final diff.
- A rebase during review costs no approval, so nothing forbids one.
- Any member of the maintainers team can queue any pull request without review, so the team holds only people trusted to merge alone.
- What lands on `main` has the approved diff, but not always the approved messages, split, SHA or signature: GitHub keeps an approval through a reword, and the rebase merge sets a new committer and drops signatures.
- If GitHub's rules loosen, an approval could outlive a change; nothing in CECH would catch it.

## Options considered

- **Squash merge, message from the pull request:** one commit per pull request hides its steps, and the message is edited at merge time, outside the reviewed diff.
- **Merge commits:** keep every step, including fix-ups, and make the history non-linear.
- **Free-form messages:** nothing for release notes to sort by.
- **Squashing before the final approval:** one more round trip for every pull request.
- **A required check that replays the approved commits:** it also catches a reworded message or another split, but needs a second list of whose approval counts, since a workflow's token cannot read team membership, and a status that any collaborator with write access could forge.
