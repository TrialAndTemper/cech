# 21. Roles

- Status: accepted
- Date: 2026-10-29
- Deciders: Michał Bożek

## Context

A skill often needs another kind of skill: release notes cite issues in the issue tracker's format.
It cannot name that skill: the project chooses its tracker, and may replace a CECH skill with an extension's ([ADR 20](0020-the-extension-model.md) item 2).
So skills compose through roles that the project binds ([ADR 8](0008-a-marketplace-you-extend-not-fork.md) item 3), and a skill names another only when it depends on that exact plugin ([ADR 11](0011-one-plugin-one-skill.md) item 3).

The [Agent Skills spec](https://agentskills.io/specification), read on 2026-10-29, makes `metadata` a map of strings and asks for key names "reasonably unique to avoid accidental conflicts".

Probed on 2026-10-29 with Claude Code 2.1.312 (haiku, sonnet) and Codex CLI 0.165.0 (gpt-6-luna, gpt-6.1-sol), three runs per model: a project with two `AGENTS.md` layers, a release-notes skill, and two tracker skills with different issue formats, of which the project chose one.
With the choice recorded nowhere, none of 12 runs used either format.
With the choice in a generated `.cech/roles.toml` that the release-notes skill points to, all 12 used the chosen format, as did all 12 with it listed in the root `AGENTS.md`.

## Decision

1. **A role** is a kind of skill that other skills rely on, such as `issue-tracker`.
   A skill fills a role when it does that job; it uses a role when its rules rely on the role's skill.
2. **Vocabulary.**
   Each marketplace lists the roles it defines, with what each means, in `data/marketplace.toml` ([ADR 17](0017-repository-data-and-state.md) item 7).
   A role takes at most one bound skill unless it is marked as taking several.
   Role names have the form of [ADR 12](0012-categories-and-names.md) item 3, never start with `cech-`, and in an extension start with its namespace.
   Roles are versioned with the marketplace release that lists them ([ADR 8](0008-a-marketplace-you-extend-not-fork.md) item 6), so removing or renaming one is a breaking change.
3. **Metadata.**
   A skill declares the roles it fills and uses in `SKILL.md` `metadata`, under the keys `cech-fills` and `cech-uses`.
   A CECH skill names only CECH's roles; an extension skill names its own and those of the CECH release it builds on.
4. **Every use is optional.**
   Where a skill relies on a role, its text names the role and tells the agent to find the bound skills in `.cech/roles.toml`.
   It also says what to do when no bound skill is available, and an unbound role is never an error.
5. **Binding.**
   For each role an enabled skill uses, `cech` binds the enabled skills that fill it, unless the project lists the skills to bind in `.cech/config.toml`, where an empty list leaves the role unbound.
   So replacing a CECH skill with an extension skill that fills the same roles needs no entry.
6. **Rendering.**
   Whenever `cech` renders the native files ([ADR 19](0019-per-project-plugin-enablement.md) item 2), it also writes the bindings, with what each role means, to a committed `.cech/roles.toml` that says it is generated and names the file to edit.
   A project with hand-written native files ([ADR 19](0019-per-project-plugin-enablement.md) item 7) may write it by hand.
7. **Checks.**
   `cech` rejects a vocabulary, metadata or binding that breaks items 2, 3 and 5, and a `.cech/roles.toml` that differs from its rendering, unless the native files are hand-written.
8. **Without its plugin**, a copy of a skill finds no bindings unless the project writes the file.
   Either way it follows item 4, so no role makes a skill need `cech`.

## Consequences

- Each skill that uses a role is evaluated with and without its bindings, on both harnesses.
- Most projects list no bindings in `.cech/config.toml`.
- Extension skills can fill CECH's roles, so they plug into CECH skills that never name them.

## Options considered

- **Required roles:** a second kind of hard dependency, and such a skill breaks without `cech`.
- **Every binding written by hand:** each replacement edits two places.
- **Every role takes many skills:** two trackers would bind silently.
- **One skill per role:** a polyglot project would need a role per language.
- **CECH's roles in `data/roles.toml`:** two formats for one kind of data.
- **Keys `fills` and `uses`:** other clients may read the same words.
- **A block of bindings in the root `AGENTS.md`:** grows a file kept short, and did no better in the probe.
- **Bindings in a generated part of `.cech/config.toml`:** mixes choices with derived output, which invites edits.
- **Skills read the config:** most bindings are derived, not written there.
