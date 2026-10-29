# 14. Releases and versions

- Status: accepted
- Date: 2026-10-28
- Deciders: Michał Bożek

## Context

CECH releases its plugins together as one catalogue, and releases its layout rules with each CLI release ([ADR 8](0008-a-marketplace-you-extend-not-fork.md) item 6).
A project pins a release by its tag, and each pinned release carries its version in its catalogue's name ([ADR 12](0012-categories-and-names.md) item 5).
Generated files must be the same in every clone, so the build cannot read the version from a tag.

Checked on 2026-10-28 with Claude Code 2.1.312: `claude plugin validate --strict` fails on a `plugin.json` with no `version`, and passes with `0.0.0-dev` and with `0.1.0-rc.1`.

The [Claude Code plugin loading docs](https://code.claude.com/docs/en/plugins/loading), read the same day, say Claude Code uses a manifest's `version` to name a plugin's cache folder and to decide whether an update changes anything.
Only a plugin with no `version` in a git marketplace takes it from the commit, and a local directory marketplace always loads current files.

## Decision

1. **Releases are cut from `main`.**
   A catalogue release gets its own commit, on top of a commit of `main` that passes the gates and holds the changelog entry.
   It changes only what a release build writes (item 3).
   Only the tag points to the release commit, so `main`'s catalogue keeps the namespace's name.
2. **Two kinds of tag.**
   - A catalogue release is tagged `vX.Y.Z`, or `vX.Y.Z-rc.N` for a pre-release (item 5), on its release commit.
   - A CLI release is tagged `cli/vX.Y.Z` or `cli/vX.Y.Z-rc.N` on a commit of `main`, with version numbers of its own.

   `X`, `Y`, `Z` and `N` are decimal with no leading zeros, and `N` starts at 1.
   A ruleset blocks moving or deleting either kind of tag, and creating a catalogue tag outside the release workflow (item 6).
3. **A release build.**
   `cech build --release V` takes `V`, a version as in item 2 without the `v`.
   It names the catalogue `<namespace>-V`, with each `.` in `V` turned into `-`, such as `cech-0-2-0-rc-1`.
   It writes `V` as the `version` of every `plugin.json`; the catalogue has none.
   Without `--release`, the catalogue keeps the namespace's name and every `version` is `0.0.0-dev`.
4. **One version for every plugin.**
   A plugin's `version` is generated, never written by hand, and there are no per-plugin versions or tags.
   The changelog, not the version, says which plugins changed.
5. **Pre-releases.**
   `rc.N` is the only pre-release form; a final release is one without it.
   Versions are ordered as SemVer says.
   A command that picks a release by default skips pre-releases.
   A final extension release builds on a final CECH release.
6. **A release workflow creates catalogue tags.**
   A maintainer starts it with `V` and a commit, and it fails unless the commit is on `main` and `V` is above every earlier catalogue release.
   It makes the release commit with `cech build --release V`, runs every gate and the private term tier ([ADR 3](0003-vendor-neutrality.md) item 4) on it, and pushes the tag only when all pass.
7. **Extensions** follow items 1 to 6 with their own namespace, tags, default branch and private term list, without CLI tags.
   On a host without rulesets or workflows, a maintainer keeps to them by hand.

## Consequences

- An unchanged plugin still gets a new version with each release, and pinned projects fetch it again.
- A git marketplace that follows a branch shows Claude Code no new version, since every commit carries `0.0.0-dev`.
  Contributors try plugins from a local directory, and users try a release at its `-rc.N` tag.
- Each release is above the last, so an older line gets no patch release.
- The release workflow's other steps (artifacts, checksums, attestations and GitHub releases), the CLI's release pipeline and the changelog's format are outside this ADR.

## Options considered

- **The build reads the version from git tags:** a shallow clone and a full clone would build different files.
- **The version in `data/marketplace.toml`:** a hand-written key that must stay off `main`, and can disagree with the tag.
- **A `cech release` command that commits and tags:** the CLI would write git history.
- **The release commit on `main`:** `main` would carry a release's name.
- **A development branch:** pinned projects see only tags, so it just adds merges.
- **Per-plugin versions and `<name>--vX.Y.Z` tags:** version bumps that no tool reads.
- **No `version` outside a release build:** strict validation fails on every pull request.
- **Tags pushed by hand and checked afterwards:** a tag that fails cannot be moved or deleted, and a project could pin it.
- **No pre-releases:** a release cannot be tried under its versioned name first.
- **One tag for the CLI and the catalogue:** a CLI fix would need a catalogue release, and Go needs `cli/` tags for the module in `cli/`.
