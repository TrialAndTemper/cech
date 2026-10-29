# 22. Two install paths

- Status: accepted
- Date: 2026-10-29
- Deciders: Michał Bożek

## Context

Skill installers can copy skills out of CECH's public repository.
A *copy* is a skill folder that an installer wrote: a *project copy* sits in a repository, a *personal copy* in a home directory.

Checked on 2026-10-28 in the source of `skills` 1.7.4, the npm package behind `npx skills`, and of its main branch at 4d786c1:

- `npx skills add <source>#<ref>` finds CECH's skills through `.claude-plugin/marketplace.json`, and copies only the skill folder, without the plugin's other parts or the root `LICENSE` and `NOTICE`.
- `skills-lock.json` records each copy's source, ref and a hash of its files, but the released restore command, `skills experimental_install`, downloads each ref again without checking the hash; a check exists only on the main branch.
- Telemetry can send source and skill names, removal included, unless `DISABLE_TELEMETRY=1` is set.
- The [skills.sh docs](https://skills.sh/docs/packs) say a pack, a shared list of skills, is "unlisted, not access-controlled": anyone with its link can install it.

Read on 2026-10-28, with Claude Code 2.1.312 and Codex CLI 0.165.0 installed:

- The [Claude Code skills docs](https://code.claude.com/docs/en/skills) name a plugin's skill `plugin:skill`, so a copy with the same skill name loads beside it.
  The `skillOverrides` setting, also allowed in project settings, turns a skill off by name; "plugin skills are not affected".
- The [Codex skills docs](https://learn.chatgpt.com/docs/build-skills) say Codex loads both skills when two share a name.
  Its only documented switch for one skill is in `~/.codex/config.toml`.

## Decision

1. **Two paths from the same skill folders.**
   - *Managed:* `cech` pins the plugins a project enables from CECH and extensions ([ADR 19](0019-per-project-plugin-enablement.md)).
   - *Standalone:* people make copies of portable skills with an installer, without `cech`.
2. **Skills are portable by default.**
   A skill must do what it claims with its own folder alone.
   It may still read its facts in `.cech/<namespace>/<plugin>.toml` ([ADR 17](0017-repository-data-and-state.md)).
3. **Managed-only skills say so.**
   A skill is managed-only when it cannot do what it claims without reviewer agents, hooks, an MCP server, a dependency or a role binding ([ADR 21](0021-roles.md)).
   Its `SKILL.md` sets `metadata.cech-managed-only: "true"`, and its text says what a copy lacks.
   `cech validate` requires the key where the plugin's files show a dependency, hooks or an MCP server; for reviewer agents and role bindings, the author decides.
4. **Licence and notices travel with the folder.**
   Each skill folder holds a copy of the root `LICENSE` when the marketplace has one ([ADR 18](0018-generated-files.md) item 2), and its own `NOTICE` when the skill holds third-party text ([ADR 2](0002-licence-and-contributions.md) item 4).
   Both count as assets of the skill ([ADR 11](0011-one-plugin-one-skill.md) item 2).
   `SKILL.md` does not link to them, so agents do not read them as guidance.
5. **No project copies of managed skills.**
   In a project with `.cech/config.toml`, `cech check` fails on a project copy of any skill from a managed marketplace ([ADR 19](0019-per-project-plugin-enablement.md) item 1), enabled or not.
6. **The plugin wins over personal copies.**
   For each enabled plugin of a managed marketplace, `cech` writes `"skillOverrides": {"<skill>": "off"}` into `.claude/settings.json`.
   Claude Code then turns off other skills of that name, not the plugin's.
   Hand-written native files ([ADR 19](0019-per-project-plugin-enablement.md) item 7) carry these entries by hand.
   Codex documents no project-level switch, so a personal copy still loads there, a gap recorded in the harness data file ([ADR 9](0009-native-harness-features-first.md) item 2).
7. **CECH promises little for installers.**
   It promises only that an installer can copy a portable skill at a release tag.
   Catalogue tags cannot move ([ADR 14](0014-releases-and-versions.md) item 2), so a tag always serves the same files.
   No released installer checks that it got them, so CECH does not treat `skills-lock.json` as a pin, and builds no lockfile or restore tool ([ADR 8](0008-a-marketplace-you-extend-not-fork.md) item 5).
8. **Private extensions stay on authenticated git** ([ADR 20](0020-the-extension-model.md) item 6).
   CECH's docs for extensions advise against skills.sh packs, and advise `DISABLE_TELEMETRY=1` for every installer command on a copy from a private source.

## Consequences

- Teams can use CECH skills without `cech`, but lose its pins, dependency checks and reviewer agents.

## Options considered

- **Managed path only, or copies allowed but unsupported:** people copy anyway, and skills that need their plugin break silently.
- **The installer's `metadata.internal: true`:** hides a skill from the installer's list, but [Agent Skills](https://agentskills.io/specification) `metadata` allows only strings, and only one installer reads it.
- **Inferring the mark instead of writing it:** misses role bindings, and is invisible in `SKILL.md`.
- **Copies of skills a managed project has not enabled:** they are not pinned, and their facts files fail `cech check` ([ADR 17](0017-repository-data-and-state.md) item 4).
- **Only the root `NOTICE`, or only `license` in `SKILL.md` frontmatter:** MIT asks for its notice in every copy, and the copy would have none.
