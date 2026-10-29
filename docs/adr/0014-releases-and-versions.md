# 14. Releases and versions

- Status: accepted
- Date: 2026-10-28
- Deciders: Michał Bożek

## Context

CECH releases its plugins together as one catalogue, and releases its layout rules with each CLI release ([ADR 8](0008-a-marketplace-you-extend-not-fork.md) item 6).
A project pins a release by its tag.
At that tag the catalogue must carry a versioned name, such as `cech-0-1-0`; on every other commit it carries the namespace's name ([ADR 19](0019-per-project-plugin-enablement.md) items 1, 3 and 6).
`cech build` must build the same files in every clone ([ADR 18](0018-generated-files.md)), so it cannot read the version from a tag.
A dependency on a CECH plugin names the CECH release it builds on, never a range of versions ([ADR 20](0020-the-extension-model.md) item 4).

Checked on 2026-10-28 with Claude Code 2.1.312:

- `claude plugin validate --strict` fails on a `plugin.json` with no `version` ("No version specified"), and passes with `0.0.0-dev` and with `0.1.0-rc.1`.

The [Claude Code plugin loading docs](https://code.claude.com/docs/en/plugins/loading), read the same day, say Claude Code uses a manifest's `version` to name a plugin's cache folder and to decide whether an update changes anything.
Only a plugin with no `version` in a git-hosted marketplace takes its version from the commit.
A local directory marketplace always loads current files.

## Decision

1. **Releases are cut from `main`.**
   A catalogue release starts from a commit on `main` that passes the gates and holds the release's changelog entry.
   The release commit sits on top of that commit and changes only what `cech build --release` writes (item 3).
   Only the tag points to it; it is never merged into `main`, so the catalogue on `main` keeps the namespace's name.
2. **Two kinds of tag.**
   - A catalogue release is tagged `vX.Y.Z`, or `vX.Y.Z-rc.N` for a pre-release (item 5), on its release commit.
   - A CLI release is tagged `cli/vX.Y.Z` or `cli/vX.Y.Z-rc.N` on a commit of `main`, with version numbers of its own.

   `X`, `Y`, `Z` and `N` are decimal with no leading zeros, and `N` starts at 1.
   A ruleset blocks moving or deleting either kind of tag, and creating a catalogue tag outside the release workflow (item 6).
3. **A release build.**
   `cech build --release V` takes `V` as `X.Y.Z` or `X.Y.Z-rc.N` (item 2), with no leading `v`, and exits 2 on any other value ([ADR 15](0015-the-cech-cli.md) item 5).
   It names the catalogue `<namespace>-V`, with each `.` in `V` turned into `-`: `cech-0-1-0`, `cech-0-2-0-rc-1`, `acme-1-4-0`.
   It writes `V` as the `version` of every `plugin.json`; the catalogue has none.
   Without `--release`, the catalogue keeps the namespace's name and every `version` is `0.0.0-dev`.
   `--check` and `--dry-run` take `--release V` too, and then check or report the release build's output.
4. **One version for every plugin.**
   A plugin's `version` is generated, never written by hand, and there are no per-plugin versions or tags.
   The changelog, not the version, says which plugins changed.
5. **Pre-releases.**
   `rc.N` is the only pre-release form; a final release is one without it.
   Versions are ordered as SemVer says: `0.2.0-rc.1` comes before `0.2.0`.
   By default, `cech init` and `cech upgrade` skip pre-releases ([ADR 19](0019-per-project-plugin-enablement.md) item 8).
   A final extension release must build on a final CECH release, so `cech build --release` with a final `V` exits 1 when the `cech` key names a pre-release.
6. **A release workflow creates catalogue tags.**
   A maintainer starts it with `V` and a commit on `main`.
   It fails when the commit is not on `main`, or when `V` is not above every earlier catalogue release.
   It makes the release commit with `cech build --release V`.
   The commit's message is `chore(release): vV`.
   The maintainer who starts the workflow is its author and signs it off; the workflow adds the `Signed-off-by:` trailer.
   The workflow then runs every gate on that commit, using `cech build --check --release V` as the build check and running the commit message gate on it alone.
   It also runs the private term tier ([ADR 3](0003-vendor-neutrality.md) item 4).
   It pushes the tag only when all of these pass.
7. **Extensions** follow items 1 to 6 with their own namespace, tags, default branch and private term list; they have no CLI tags.
   Rulesets apply where the host has them.
   On a host without workflows, a maintainer runs item 6's steps by hand.

## Consequences

- A plugin that does not change between releases still gets a new version, and pinned projects fetch it again.
- A git marketplace that follows a branch does not pick up new commits in Claude Code, because every commit carries `0.0.0-dev`.
  Contributors try plugins through a local directory marketplace or `--plugin-dir`, and users try a release through its `-rc.N` tag.
- Each release is above the last, so an older line gets no patch release.
- CECH needs the release workflow before its first release.
  Its other steps (artifacts, checksums, attestations and GitHub releases), the CLI's release pipeline and the changelog's format are left to a later ADR.

## Options considered

- **The build reads the version from git tags:** a shallow clone and a full clone would build different files.
- **The version in `data/marketplace.toml`:** a hand-written key that must stay off `main`, and can disagree with the tag.
- **A `cech release` command that commits and tags:** the CLI would write git history.
- **The release commit on `main`:** [ADR 19](0019-per-project-plugin-enablement.md) item 6 keeps versioned names off `main`.
- **A development branch:** pinned projects see only tags, so it just adds merges.
- **Per-plugin versions and `<name>--vX.Y.Z` tags:** version bumps that no tool reads.
- **No `version` outside a release build:** strict validation fails on every pull request, unless `cech validate` skips that check; revisit if `--strict` stops requiring it.
- **Tags pushed by hand and checked afterwards:** a tag that fails cannot be moved or deleted, and `cech init` could pin it.
- **No pre-releases:** a release cannot be tried under its versioned name first.
- **One tag for the CLI and the catalogue:** a CLI fix would need a catalogue release, and Go needs `cli/` tags for the module in `cli/`.
