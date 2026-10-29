# 10. Claude Code and Codex from one source

- Status: accepted
- Date: 2026-10-16
- Deciders: Michał Bożek

## Context

CECH serves Claude Code and Codex, and should not write everything twice.
Checked on 2026-10-16 (Claude Code 2.1.301, Codex CLI 0.162.1):

- Both read the [Agent Skills](https://agentskills.io/specification) `SKILL.md` format.
- Codex installs a marketplace and plugins that have only Claude Code manifests (`.claude-plugin/*.json`).
  It puts their skills in the prompt, runs their MCP servers, and runs their hooks once trusted.
- Codex ignores agent definitions bundled in a plugin, and has no plugin `validate` or `eval` command.
- Claude Code enforces plugin `dependencies` but does not install them.
  Codex ignores the field.
- [Codex docs](https://developers.openai.com/plugins/build/plugins) call the Claude marketplace format "legacy-compatible" but announce no removal.
  Claude Code does not document the newer Agent Plugins format and loads such packages only in part.
- A `plugin.json` at a plugin's root makes Codex use its Agent Plugins loader, which skips hooks.

## Decision

1. The source is the Claude Code plugin layout plus Agent Skills `SKILL.md`.
   No hand-written Codex manifests or catalogues, and no `plugin.json` at a plugin's root; the validator rejects one.
   A description lives in `SKILL.md` only; other copies are generated.
2. Frontmatter uses Agent Skills spec fields, unless a Claude-only field is needed.
   The validator warns on each such field.
3. Codex extras are generated, and only where Codex needs them.
   The build writes reviewer agents as Codex TOML, and the `cech` CLI copies those of enabled plugins into the project's `.codex/agents/`.
   Both steps are one harness-gap workaround under [ADR 9](0009-native-harness-features-first.md), not an installer.
   The copies are git-ignored, so the project still commits only what it enables ([ADR 8](0008-a-marketplace-you-extend-not-fork.md)).
   The workaround ends when a probe shows Codex loads plugin-bundled agents.
4. A skill's claims hold on both harnesses, unless the skill names the one harness it is limited to.
   Evals run on both; how they run is outside this ADR.
5. The scheduled job ([ADR 9](0009-native-harness-features-first.md)) checks that the newest Codex still loads a Claude-shaped test plugin, and that it prints no deprecation warning for it.
   If either check fails, a new ADR chooses between a Codex generator and Agent Plugins as the source.

## Consequences

- CECH depends on Codex staying compatible; the scheduled job warns early.
- Some Claude-only features need extra work or go unused.

## Options considered

- **Two hand-written layouts plus a sync lint:** one description in many files, which drift.
- **Agent Plugins as the source:** Claude Code does not document it.
- **Claude Code only:** leaves out Codex users.
