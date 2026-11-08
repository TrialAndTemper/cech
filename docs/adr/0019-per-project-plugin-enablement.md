# 19. Per-project plugin enablement

- Status: accepted
- Date: 2026-10-26
- Deciders: Michał Bożek

## Context

A project enables plugins from CECH and from extensions ([ADR 8](0008-a-marketplace-you-extend-not-fork.md)), each at a version it chooses.
Claude Code reads them from `.claude/settings.json` and Codex from `.codex/config.toml`, so the two files must agree.
This ADR covers plugins, not skills installed without them.

Probed on 2026-10-16 (Claude Code 2.1.301) and 2026-10-21 (Claude Code 2.1.303), both with Codex CLI 0.162.1, with local git marketplaces:

- Both harnesses load a plugin that a trusted project's settings declare and enable, with no user install.
- A plugin enabled in user settings loads in every project that does not set it to `false`, even when a catalogue's `defaultEnabled` is `false`.
- Each harness keeps one copy of a marketplace per name per user, so projects that pin one name at different tags refetch at each switch, and in the probe got each other's version.
- Codex refuses a marketplace whose catalogue `name` differs from the declared one; a tag whose catalogue is named `acme-1-0-0` loads under that name in both.

The [Claude Code marketplace docs](https://code.claude.com/docs/en/plugins/marketplace-reference), read on 2026-10-16, accept only a branch or tag as a marketplace's `ref`.

## Decision

1. **One config file.**
   `.cech/config.toml` ([ADR 17](0017-repository-data-and-state.md)) lists the marketplaces the project manages, its *managed marketplaces*, each with its source and the catalogue release tag it pins ([ADR 14](0014-releases-and-versions.md) item 2), and the plugins the project enables from each, by namespace ([ADR 12](0012-categories-and-names.md) items 4 and 5).
   `cech` writes each pin, together with the commit the tag pointed to when pinned.
   Other marketplaces go into the harnesses' settings files by hand, and `cech` leaves them as they are.
   A source never holds credentials; access uses the machine's git configuration.
2. **Rendered native files.**
   `cech` renders the marketplace and plugin entries of both *native files*, `.claude/settings.json` and `.codex/config.toml`, from the config, leaving every other byte as it was ([ADR 15](0015-the-cech-cli.md) item 3); the project commits them.
3. **Exact pins.**
   A project declares each marketplace under the `name` in its catalogue at the pinned commit, the versioned name of [ADR 14](0014-releases-and-versions.md) item 3, such as `acme-1-4-0`.
   Keys use it, `cech-prose@cech-0-1-0`, so versions sit side by side.
4. **A complete map.**
   For each managed marketplace, the native files hold `true` for each enabled plugin and `false` for every other plugin in its catalogue at the pinned commit.
   Apart from item 6, `cech` writes no keys for other marketplaces.
5. **Dependencies.**
   Neither harness installs dependencies ([ADR 10](0010-claude-code-and-codex-from-one-source.md)), so enabling a plugin also enables what it depends on, and fails when a managed catalogue at its pinned commit lacks it; disabling a plugin that another enabled plugin needs is refused.
6. **Unversioned copies.**
   Outside a release tag a catalogue is named after its namespace ([ADR 12](0012-categories-and-names.md) items 4 and 5).
   So the native files also set every plugin of each pinned catalogue to `false` under the bare namespace, such as `cech-prose@cech`.
   A copy enabled in user settings then cannot load beside the pin.
   `cech` never writes to user settings.
7. **Hand-written files.**
   A project may write the native files by hand, without the config, following items 3, 4 and 6, with each dependency enabled; `cech` then neither renders them nor checks the pins or catalogue names.
8. **Commands.**
   `cech` commands write the config and render the native files from it; a read-only check fails when the files break this ADR or two enabled plugins share a name ([ADR 12](0012-categories-and-names.md) item 7).
   A command run in each clone makes Codex install every enabled plugin at its pin, a gap recorded in the harness data file ([ADR 9](0009-native-harness-features-first.md) item 2), and syncs their reviewer agents ([ADR 10](0010-claude-code-and-codex-from-one-source.md) item 3).

## Consequences

- An upgrade changes every key in one reviewable diff.
- Pinned versions cost disk space.
- `cech check` reads each catalogue at its pinned commit and does not pass when it cannot reach a source ([ADR 15](0015-the-cech-cli.md) item 5).
- A copy enabled in user settings under another versioned name still loads beside the pin, and `cech check` misses it.

## Options considered

- **Hand-written native files only:** two files to keep in step.
- **One unversioned name per marketplace:** the refetching and mixed versions above.
- **Only `true` keys:** user-enabled plugins leak in.
- **A CECH marketplace that `cech` enables in user settings:** a write to user settings and a second release stream, while item 6 already guards pinned projects.
- **Pin by commit:** the docs accept no commit as a marketplace's `ref`.
- **Dependencies left to the harnesses:** neither installs them.
