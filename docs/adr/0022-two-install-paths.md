# 22. Two install paths

- Status: accepted
- Date: 2026-10-29
- Deciders: Michał Bożek

## Context

Skill installers can copy skills out of CECH's public repository, and no ADR says what such a copy may rely on.
A *copy* is a skill folder that an installer wrote: a *project copy* sits in a repository, a *personal copy* in a home directory.
A *managed marketplace* is one that a project's `.cech/config.toml` lists ([ADR 19](0019-per-project-plugin-enablement.md) item 1).

Checked on 2026-10-28 in the source of `skills` 1.7.4, the npm package behind `npx skills`, and of its main branch at 4d786c1:

- `npx skills add <source>#<ref>` finds CECH's skills through `.claude-plugin/marketplace.json`.
- It copies only the skill folder, without the plugin's other parts or the root `LICENSE` and `NOTICE`.
- A project copy goes in `.agents/skills/`, where Codex looks.
  By default, Claude Code gets a symlink to it in `.claude/skills/`.
- `skills-lock.json` records each copy's source, ref and a hash of its files.
- The restore command, `skills experimental_install`, downloads each recorded ref again and does not check the hash.
  A hash check, `--frozen-lockfile`, is on the main branch but not in a release.
- Telemetry can send source and skill names: on install for sources not on GitHub, and on `skills remove` for every source.
  `DISABLE_TELEMETRY=1` stops it.
- The [skills.sh docs](https://skills.sh/docs/packs) say a pack, a shared list of skills, is "unlisted, not access-controlled": anyone with its link can install it.

Read on 2026-10-28, with Claude Code 2.1.312 and Codex CLI 0.165.0 installed:

- The [Claude Code skills docs](https://code.claude.com/docs/en/skills) name a plugin's skill `plugin:skill`, so a copy with the same skill name loads beside it.
  The `skillOverrides` setting, also allowed in project settings, turns a skill off by name; "plugin skills are not affected".
- The [Codex skills docs](https://learn.chatgpt.com/docs/build-skills) say Codex loads both skills when two share a name.
  Its only documented switch for one skill is in `~/.codex/config.toml`.

## Decision

1. **Two paths from the same skill folders.**
   - *Managed:* `cech` pins the plugins a project enables from CECH and extensions ([ADR 8](0008-a-marketplace-you-extend-not-fork.md), [ADR 19](0019-per-project-plugin-enablement.md)).
   - *Standalone:* people make copies of portable skills with an installer, without `cech`.
2. **Skills are portable by default.**
   A skill must do what it claims with its own folder alone.
   It may still read its facts in `.cech/<namespace>/<plugin>.toml` ([ADR 17](0017-repository-data-and-state.md)).
3. **Managed-only skills say so.**
   A skill is managed-only when it cannot do what it claims without reviewer agents, hooks, an MCP server, a dependency or a role binding ([ADR 8](0008-a-marketplace-you-extend-not-fork.md) item 3).
   Its `SKILL.md` sets `metadata.cech-managed-only: "true"`, and its text says what a copy lacks.
   `cech validate` rejects any other value.
   It also requires the key when the plugin's files show the need: a dependency in `plugin.json`, or hooks or an MCP server in `plugin.json`, `hooks/hooks.json` or `.mcp.json`.
   For reviewer agents and role bindings, the author decides.
4. **Licence and notices travel with the folder.**
   Each skill folder holds a copy of the root `LICENSE` ([ADR 18](0018-generated-files.md) item 2) and its own `NOTICE` ([ADR 2](0002-licence-and-contributions.md) item 4).
   Both count as assets of the skill ([ADR 11](0011-one-plugin-one-skill.md) item 2).
   `SKILL.md` does not link to them, so agents do not read them as guidance.
5. **No project copies of managed skills.**
   In a project with `.cech/config.toml`, `cech check` fails on a project copy of any skill from a managed marketplace.
   Symlinks count, and so do skills whose plugin is not enabled.
   `cech check` looks in every `.agents/skills/` and `.claude/skills/` folder in the working tree, git-ignored ones included.
   It matches by the `name` in `SKILL.md`, or by folder name when it cannot read one.
6. **The plugin wins over personal copies.**
   For each enabled plugin of a managed marketplace, `cech` writes `"skillOverrides": {"<skill>": "off"}` into `.claude/settings.json`.
   Claude Code then turns off other skills of that name, not the plugin's.
   `cech` adds or removes an entry as its plugin is enabled or disabled, and leaves other entries alone ([ADR 15](0015-the-cech-cli.md) item 3).
   Hand-written native files ([ADR 19](0019-per-project-plugin-enablement.md) item 7) carry these entries by hand, and in projects that use them `cech` skips items 5 and 6.
   Codex documents no project-level switch, so a personal copy still loads there; that gap goes in `data/harnesses.toml` ([ADR 9](0009-native-harness-features-first.md) item 2).
7. **CECH promises little for installers.**
   It promises only that an installer can copy a portable skill at a release tag, such as `npx skills add TrialAndTemper/cech#v0.1.0 --skill cech-prose`.
   Catalogue tags cannot move ([ADR 14](0014-releases-and-versions.md) item 2), so a tag always serves the same files.
   No released installer checks that it got them, so CECH does not treat `skills-lock.json` as a pin, and builds no lockfile or restore tool ([ADR 8](0008-a-marketplace-you-extend-not-fork.md) item 5).
   This is revisited when a released installer checks the hash.
8. **Private extensions stay on authenticated git** ([ADR 20](0020-the-extension-model.md) item 7).
   CECH's docs for extensions will advise against skills.sh packs for their skills.
   They will also advise `DISABLE_TELEMETRY=1` for every installer command that handles a copy from a private source, including removal.

## Consequences

- Teams can use CECH skills without `cech`, but lose its pins, dependency checks and reviewer agents.
- To check with the first plugin and a pinned installer, recording versions:
  1. a copy holds the skill's `LICENSE` and `NOTICE`; if not, item 4 changes;
  2. the skill does what it claims on both harnesses, as a copy and as a plugin; if not, it is managed-only;
  3. a restore at a tag gives the same files as the first copy; if not, CECH's docs warn against restores;
  4. project `skillOverrides` turns off project and personal copies but not the plugin; if not, item 6 changes.

## Options considered

- **Managed path only, or copies allowed but unsupported:** people copy anyway, and skills that need their plugin break silently.
- **The installer's `metadata.internal: true`:** hides a skill from the installer's list, but [Agent Skills](https://agentskills.io/specification) `metadata` allows only strings, and only one installer reads it.
- **Inferring the mark instead of writing it:** misses role bindings, and is invisible in `SKILL.md`.
- **Copies of skills a managed project has not enabled:** they are not pinned, and their facts files fail `cech check` ([ADR 17](0017-repository-data-and-state.md) item 4).
- **Only the root `NOTICE`, or only `license` in `SKILL.md` frontmatter:** MIT asks for its notice in every copy, and the copy would have none.
