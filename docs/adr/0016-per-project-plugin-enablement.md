# 16. Per-project plugin enablement

- Status: accepted
- Date: 2026-10-26
- Deciders: Michał Bożek

## Context

A project enables plugins from CECH and from extensions ([ADR 3](0003-a-marketplace-you-extend-not-fork.md)), each at a version it chooses.
Claude Code reads them from `.claude/settings.json` and Codex from `.codex/config.toml`, so the two files must agree.
This ADR covers plugins only; skills installed without their plugins are outside it.

Probed on 2026-10-16 (Claude Code 2.1.301) and 2026-10-21 (Claude Code 2.1.303), both with Codex CLI 0.162.1, with local git marketplaces:

- In a trusted project, both harnesses load a relative-path plugin its settings declare and enable, with no user install.
- A plugin enabled in user settings loads in every project unless the project sets it to `false`, even when the project does not declare its marketplace.
  A catalogue's `defaultEnabled: false` does not stop it; a plugin that no settings file names stays off.
- Each harness keeps one copy of a marketplace per name per user.
  Two projects pinning one name at different tags make both harnesses fetch again at each switch; in the probe, both served one project the other's version.
- Codex refuses a marketplace whose catalogue `name` differs from the declared name.
  A tag whose catalogue is named `acme-1-0-0` loads under that name in both harnesses.
- Two enabled plugins with one name collide silently: Claude Code loads one, chosen by key order; Codex loads both under one skill name.
- Claude Code skips a plugin whose dependency is not enabled and installs nothing; Codex ignores dependencies ([ADR 6](0006-claude-code-and-codex-from-one-source.md)).
- Codex installs a newly enabled plugin only when its marketplace's tag changes or its fetched copy is re-synced; `codex plugin add` writes user settings.
- Headless Claude Code loads project plugins only with `CLAUDE_CODE_SYNC_PLUGIN_INSTALL=1`; an interactive session needs `/reload-plugins` after a clone or pin change.

The [Claude Code marketplace docs](https://code.claude.com/docs/en/plugins/marketplace-reference), read on 2026-10-16, accept only a branch or tag as a marketplace's `ref`.

## Decision

1. **One config file.**
   `.cech/config.toml` ([ADR 15](0015-repository-state-under-cech.md)) lists the marketplaces the project manages and the plugins it enables from each:

   ```toml
   [marketplaces.cech]
   source = "https://github.com/TrialAndTemper/cech.git"
   ref = "v0.1.0"
   commit = "<40-hex commit of v0.1.0>"

   [marketplaces.acme]
   source = "git@git.acme.example:platform/acme-skills.git"
   ref = "v1.4.0"
   commit = "<40-hex commit of v1.4.0>"

   [enable]
   cech = ["cech-conventional-commits", "cech-prose"]
   acme = ["acme-tracker"]
   ```

   Each key under `marketplaces` and `enable` is a namespace ([ADR 8](0008-categories-and-names.md) items 4 and 5).
   `ref` is a release tag (`v`, then `X.Y.Z` in decimal with no leading zeros, and optionally `-rc.N` for a pre-release, with `N` from 1) and `commit` the tag's commit at pinning; `cech` writes both.
   The config manages only CECH and extensions; other marketplaces go into the native files by hand, and `cech` leaves them as they are (items 2 and 4).
   A source never holds credentials; access uses the machine's git configuration.
2. **Rendered native files.**
   `cech` renders both settings files' marketplace and plugin entries from the config, leaving every other byte as it was ([ADR 14](0014-the-cech-cli.md) item 3); the project commits them.
   A later ADR on install paths adds skill entries that `cech` renders into `.claude/settings.json`.
3. **Exact pins.**
   A project declares each marketplace under the `name` in its catalogue at the pinned commit.
   Each CECH release names its catalogue `cech-` plus its version, dots as hyphens: `cech-0-1-0`, or `cech-0-2-0-rc-1` for `v0.2.0-rc.1` ([ADR 8](0008-categories-and-names.md) item 5).
   Keys use it, `cech-prose@cech-0-1-0`, so versions sit side by side.
   Each extension release names its catalogue the same way, after its namespace: `acme-1-4-0`.
4. **A complete map.**
   For each managed marketplace, the native files hold `true` for each enabled plugin and `false` for every other plugin in its catalogue at the pinned commit.
   Apart from item 6, `cech` writes no keys for other marketplaces.
5. **Dependencies.**
   `cech enable` also enables what an enabled plugin depends on, and fails when no managed catalogue at its pinned commit carries the plugin under the marketplace name the dependency gives, or under the depending plugin's own when it gives none.
   `cech disable` refuses to turn off a plugin another enabled plugin needs, and names the plugins that need it.
6. **Unversioned copies.**
   Outside a release tag, such as on a branch, a catalogue is named after its namespace ([ADR 8](0008-categories-and-names.md) items 4 and 5).
   For a managed marketplace pinned under a versioned name, the native files also hold `false` for each plugin of its pinned catalogue under the namespace, such as `cech-prose@cech`, so a copy enabled in user settings does not load beside the pin.
   `cech` never writes to user settings.
7. **Hand-written files.**
   A project may write the native files by hand, without the config, following items 3, 4 and 6, with each dependency enabled.
   Item 8's rendering and pin checks, catalogue names included, then do not apply.
8. **Commands.**
   - `cech init` writes the config with CECH pinned to its newest final release (one without `-rc.N`) unless told otherwise, renders the native files, and prints the steps each clone needs.
   - `cech enable` and `cech disable` change the plugin lists, apply item 5, and render again.
   - `cech upgrade` updates a pin, the marketplace's name and its keys together.
     Unless told a version, it moves the pin to the newest final release, and leaves a pin already at or above that release unchanged.
   - `cech check` is read-only and fails when native files differ from the rendering, a `ref` is not a release tag or no longer points at its recorded commit, a pinned catalogue is not named after its namespace and the tag's version, two enabled plugins share a name ([ADR 8](0008-categories-and-names.md) item 7), a dependency is not enabled ([ADR 7](0007-one-plugin-one-skill.md) item 3), or a source holds credentials.
   - `cech setup` runs `codex plugin marketplace upgrade` ([ADR 14](0014-the-cech-cli.md) item 2) and syncs the enabled plugins' reviewer agents ([ADR 6](0006-claude-code-and-codex-from-one-source.md) item 3).

## Consequences

- An upgrade changes every key in one reviewable diff.
- Pinned versions cost disk space.
- `cech check` reads each catalogue at its pinned commit and does not pass when it cannot reach a source ([ADR 14](0014-the-cech-cli.md) item 5).
- A copy enabled in user settings under another versioned name still loads beside the pin, and `cech check` misses it.
- Each person trusts the project in both harnesses once per clone, and runs `cech setup` after each clone and config change.
  `/reload-plugins`, `CLAUDE_CODE_SYNC_PLUGIN_INSTALL` and the Codex re-sync for a newly enabled plugin, barred to `cech` (item 6 here; [ADR 14](0014-the-cech-cli.md) item 2), are documented; each workaround goes in the harness data file ([ADR 5](0005-native-tool-features-first.md) item 2).

## Options considered

- **Hand-written native files only:** two files to keep in step.
- **One unversioned name per marketplace:** the refetching and mixed versions above.
- **Only `true` keys:** user-enabled plugins leak in.
- **A CECH marketplace that `cech` enables in user settings:** a write to user settings and a second release stream, while item 6 already guards pinned projects.
- **Pin by commit:** the docs accept no commit as a marketplace's `ref`.
- **Dependencies left to the harnesses:** neither installs them.
