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
- **Gate:** a check that a change must pass before it merges ([ADR 5](0005-gates-and-tool-pins.md)).
- **Probe:** a repeatable check of how a harness behaves ([ADR 9](0009-native-harness-features-first.md)).
- **Managed marketplace:** one that a project's `.cech/config.toml` lists ([ADR 19](0019-per-project-plugin-enablement.md)).
- **Native files:** the harnesses' project settings, `.claude/settings.json` and `.codex/config.toml` ([ADR 19](0019-per-project-plugin-enablement.md)).

## Records

1. [Record architecture decisions](0001-record-architecture-decisions.md)
   - How an ADR is written, numbered, accepted and changed.
2. [Where each kind of text lives](0002-where-each-kind-of-text-lives.md)
   - Where contributor rules, the pages on what CECH does now, and a change's working material live.
3. [Licence and contributions](0003-licence-and-contributions.md)
   - The MIT licence, DCO sign-off, consent before shipping, and the rules for third-party text.
4. [Vendor neutrality](0004-vendor-neutrality.md)
   - Nothing specific to a private organisation, invented examples, and the term check that enforces it.
5. [Gates and tool pins](0005-gates-and-tool-pins.md)
   - mise as the one entry point, CI as the authority, the gates, and exact tool pins.
6. [Change record](0006-change-record.md)
   - Rebase merges, commits as the record, Conventional Commits and sign-off.
7. [Markdown source format](0007-markdown-source-format.md)
   - Markdown with one sentence per line, checked by rumdl.
8. [A marketplace you extend, not fork](0008-a-marketplace-you-extend-not-fork.md)
   - CECH is a marketplace of skills for Claude Code and Codex that organisations extend in their own repositories.
9. [Native harness features first](0009-native-harness-features-first.md)
   - Use what the harnesses already do, and record each workaround and each behaviour CECH relies on with a probe.
10. [Claude Code and Codex from one source](0010-claude-code-and-codex-from-one-source.md)
    - One source for both harnesses: the Claude Code plugin layout, with Codex extras generated.
11. [One plugin, one skill](0011-one-plugin-one-skill.md)
    - Each plugin holds one skill of the same name.
12. [Categories and names](0012-categories-and-names.md)
    - A closed list of categories, and plugin names that start with the namespace.
13. [Attribution](0013-attribution.md)
    - Who owns and maintains a plugin, and where credit for ideas goes.
14. [Releases and versions](0014-releases-and-versions.md)
    - Releases cut from `main`, their tags, one version for every plugin, and pre-releases.
15. [The cech CLI](0015-the-cech-cli.md)
    - The `cech` CLI: Go, built on the harnesses' own commands, safe edits, and output as a contract.
16. [Building the cech CLI](0016-building-the-cech-cli.md)
    - The CLI's dependencies, versions and gates.
17. [Repository data and state](0017-repository-data-and-state.md)
    - A project's CECH files live in `.cech/`, one TOML file per owner.
18. [Generated files](0018-generated-files.md)
    - `cech build` generates every derived file from `SKILL.md` and `plugin.json`, and a gate checks them.
19. [Per-project plugin enablement](0019-per-project-plugin-enablement.md)
    - A project pins the marketplaces and plugins it enables in one config, and `cech` renders the harnesses' files from it.
20. [The extension model](0020-the-extension-model.md)
    - An extension is a marketplace with CECH's layout that replaces or depends on CECH's plugins, never patches them.
21. [Roles](0021-roles.md)
    - Roles let a skill rely on a kind of skill, bound per project.
22. [Two install paths](0022-two-install-paths.md)
    - Skills install as plugins or as plain copied folders.
23. [Forking upstream skills](0023-forking-upstream-skills.md)
    - How a skill forked from an upstream project keeps its notices and takes in updates.
24. [What a version change means](0024-what-a-version-change-means.md)
    - What each kind of release may change, for the catalogue and the CLI.
