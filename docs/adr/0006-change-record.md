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

## Decision

1. **Rebase merge only.**
   `main` takes only rebase merges, so its history stays linear, and refuses force pushes; the host enforces both.
2. **Commits are the record.**
   Anything that must last (the reason for a change, evidence, rejected options) goes in a commit message or an ADR.
   The pull request description summarises and links; it is not the only place for any of that.
   A link in a commit message points only to a public page that lasts.
   Each commit is one logical change, and review fixes are folded into the commits they fix before merge.
3. **Messages follow [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/),** with a fixed set of types, so release notes can sort commits by kind.
   The scope is for readers: a plugin may share a name with an area, so tools that need a commit's plugins read the paths it changes.
4. **Every commit is signed off by its author** ([ADR 2](0002-licence-and-contributions.md)), since the DCO is the author's statement.
   A gate checks each commit's header and sign-off, and that no fixup or merge commit reaches `main`; the rest of the message rules are for reviewers.

## Consequences

- The reason for each change is in its commit.
- Authors spend some time shaping commits before merge.
- Pull request reviewers read commits, not only the final diff.

## Options considered

- **Squash merge, message from the pull request:** one commit per pull request hides its steps, and the message is edited at merge time, outside the reviewed diff.
- **Merge commits:** keep every step, including fix-ups, and make the history non-linear.
- **Free-form messages:** nothing for release notes to sort by.
