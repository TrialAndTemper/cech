# 11. Change record

- Status: accepted
- Date: 2026-10-17
- Deciders: Michał Bożek

## Context

People reading the history later need to know what changed and why.
Pull request text lives on GitHub, not in git: forks, mirrors and offline clones do not have it.
Links to pages that need a login or expire break the same way.
Squash merges fold a pull request into one commit and lose its steps; merge commits make the history hard to read and bisect.
Release notes (a later ADR on releases) need commits they can sort by kind.

## Decision

1. **Rebase merge only.**
   Merge commits and squash merges are turned off in the repository settings, and a ruleset requires linear history on `main`.
   Force pushes to `main` are blocked.
2. **Commits are the record.**
   Anything that must last (the reason for a change, evidence, rejected options) goes in a commit message or an ADR.
   The pull request description summarises and links; it is not the only place for any of that.
3. **Clean before merge.**
   Each commit is one logical change and should leave the tree passing the gates ([ADR 10](0010-gates-and-ci.md)).
   CI runs them on the pull request as a whole, not on each commit, so this is guidance.
   Review fixes are pushed as `git commit --fixup` commits and folded in with `git rebase --autosquash` before merge, after rebasing on `main`.
   Force pushes to one's own pull request branch are expected.
4. **Message format:** [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/).
   - Header `type(scope): summary`, at most 72 characters, imperative, no final full stop.
   - Types: `feat`, `fix`, `docs`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`, `revert`.
     A revert uses `revert:` with the reverted header, not git's default `Revert "…"`.
   - Scope: the plugin name for a change inside one plugin and its evals; otherwise an area such as `cli`, `adr`, `ci`, `checks`.
     Omitted when the change spans areas.
     The scope is for readers: a plugin may share a name with an area, so tools that need a commit's plugins read the paths it changes.
   - A breaking change uses `!` after the scope and a `BREAKING CHANGE:` footer saying what users must do.
   - The body says why, wrapped at 72 characters.
5. **Trailers.**
   `Signed-off-by:` on every commit ([ADR 2](0002-licence-and-contributions.md)).
   `Co-authored-by:` for each other author, human or tool.
   No other content limits beyond [ADR 4](0004-vendor-neutrality.md).
6. **Messages stand alone.**
   A link in a commit message points only to a public, durable page: a CECH issue or pull request, a spec, an upstream commit.
7. **The commit message gate** checks every commit in a pull request for the item 4 header and a `Signed-off-by:` trailer.
   It does not apply the header rule to `fixup!` and `squash!` commits, but fails while any remain, so the pull request cannot merge until they are folded in.
   `Co-authored-by:` and item 6 are for reviewers to check.

## Consequences

- The reason for each change is in its commit.
- Authors spend some time shaping commits before merge.
- Pull request reviewers read commits, not only the final diff.

## Options considered

- **Squash merge, message from the pull request:** one commit per pull request hides its steps, and the message is edited at merge time, outside the reviewed diff.
- **Merge commits:** keep every step, including fix-ups, and make the history non-linear.
- **Free-form messages:** nothing for release notes to sort by.
