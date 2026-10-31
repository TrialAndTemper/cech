# Architecture decision records

[ADR 1](0001-record-architecture-decisions.md) sets the form and rules of these records.

## Terms

Words the ADRs use, in short:

- **Harness:** a program that runs an agent, here Claude Code or Codex ([ADR 8](0008-a-marketplace-you-extend-not-fork.md)).
- **Skill:** a folder with a `SKILL.md` that tells an agent how to do one kind of work.
- **Plugin:** the package a harness installs; in CECH each holds one skill ([ADR 11](0011-one-plugin-one-skill.md)).
- **Marketplace:** a repository or directory of plugins; CECH is one, and each extension is another.
- **Catalogue:** the file that lists a marketplace's plugins, `.claude-plugin/marketplace.json`.
- **Namespace:** a marketplace's short name, which starts each of its plugin names ([ADR 12](0012-categories-and-names.md)).
- **Role:** a kind of skill that other skills rely on, such as the issue tracker ([ADR 21](0021-roles.md)).
- **Extension:** an organisation's own marketplace, built on CECH ([ADR 20](0020-the-extension-model.md)).
- **Gate:** a check that a change must pass before it merges ([ADR 4](0004-gates-and-ci.md)).
- **Probe:** a repeatable check of how a harness behaves ([ADR 9](0009-native-harness-features-first.md)).
- **Managed marketplace:** one that a project's `.cech/config.toml` lists ([ADR 19](0019-per-project-plugin-enablement.md)).
- **Native files:** the harnesses' project settings, `.claude/settings.json` and `.codex/config.toml` ([ADR 19](0019-per-project-plugin-enablement.md)).

## Records

1. [Record architecture decisions](0001-record-architecture-decisions.md)
2. [Licence and contributions](0002-licence-and-contributions.md)
3. [Vendor neutrality](0003-vendor-neutrality.md)
4. [Gates and CI](0004-gates-and-ci.md)
5. [Tool pins](0005-tool-pins.md)
6. [Change record](0006-change-record.md)
7. [Markdown source format](0007-markdown-source-format.md)
8. [A marketplace you extend, not fork](0008-a-marketplace-you-extend-not-fork.md)
9. [Native harness features first](0009-native-harness-features-first.md)
10. [Claude Code and Codex from one source](0010-claude-code-and-codex-from-one-source.md)
11. [One plugin, one skill](0011-one-plugin-one-skill.md)
12. [Categories and names](0012-categories-and-names.md)
13. [Attribution](0013-attribution.md)
14. [Releases and versions](0014-releases-and-versions.md)
15. [The cech CLI](0015-the-cech-cli.md)
16. [Building the cech CLI](0016-building-the-cech-cli.md)
17. [Repository data and state](0017-repository-data-and-state.md)
18. [Generated files](0018-generated-files.md)
19. [Per-project plugin enablement](0019-per-project-plugin-enablement.md)
20. [The extension model](0020-the-extension-model.md)
21. [Roles](0021-roles.md)
22. [Two install paths](0022-two-install-paths.md)
23. [Forking upstream skills](0023-forking-upstream-skills.md)
24. [The term check engine](0024-the-term-check-engine.md)
