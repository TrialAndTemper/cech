# 11. One plugin, one skill

- Status: accepted
- Date: 2026-10-16
- Deciders: Michał Bożek

## Context

Projects enable whole plugins, not single skills: neither harness offers a practical per-project switch for one skill inside a plugin (checked on 2026-10-16, versions as in [ADR 10](0010-claude-code-and-codex-from-one-source.md)).
A plugin with several skills forces all of them on a project that wants one: more context, more false triggers, and blurred evals and ownership.

## Decision

1. Each CECH plugin holds exactly one skill, with the same name as the plugin.
   The validator rejects any other shape.
2. Besides the skill, a plugin may hold only these, each serving the skill:
   - read-only reviewer agents that check work against it;
   - scripts and assets in the skill folder;
   - hooks or an MCP server, if the skill cannot work without them, with the reason stated in the skill.
3. A skill that depends on another kind of skill names a role ([ADR 8](0008-a-marketplace-you-extend-not-fork.md)).
   Only when it needs one exact skill does it declare that plugin as a dependency.
   Neither harness installs dependencies ([ADR 10](0010-claude-code-and-codex-from-one-source.md)), so `cech check` reports a missing one.
   A skill never copies another skill's text, which would drift.

## Consequences

- Each skill has its own evals.
- More plugins, so naming and categories matter.
- A project enables more keys.

## Options considered

- **Themed bundles of skills, or one plugin per topic:** all-or-nothing enablement.
- **Several skills with a size limit:** the problem is enablement, not size.
