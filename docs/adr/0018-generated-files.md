# 18. Generated files

- Status: accepted
- Date: 2026-10-17
- Deciders: Michał Bożek

## Context

Claude Code and Codex need some facts about a plugin in several files: the description in `SKILL.md`, in `plugin.json` and in the marketplace catalogue; the category in the folder and in the catalogue; reviewer agents as Markdown and, for Codex, as TOML ([ADR 10](0010-claude-code-and-codex-from-one-source.md)).
Copies kept by hand drift, and a drifted catalogue shows users a wrong description or loads a wrong file.

Checked on 2026-10-17 (Claude Code 2.1.302): `claude plugin validate --strict` fails when `plugin.json` has no `description`, so the field cannot just be left out.
Checked again on 2026-10-28 (Claude Code 2.1.312): it also fails when `plugin.json` has no `version`, and Claude Code uses `version` to decide whether an update changes anything.

## Decision

1. Per plugin, two files hold hand-written metadata:
   - `SKILL.md`: name, description, `metadata`, and the skill itself;
   - `.claude-plugin/plugin.json`: every field (name, author, licence, dependencies) except `description`, `version` and, in an extension, the `marketplace` of a dependency on a CECH plugin.

   The name appears in both files and as the skill and plugin folder names.
   The validator checks that all of them match, extending the same-name rule of [ADR 11](0011-one-plugin-one-skill.md).

   Repo-wide data in `data/` is hand-written too; it describes the catalogue, not one plugin.
2. Everything else derived from them is generated:
   - the `description` field in each `plugin.json`, copied from `SKILL.md`;
   - the `version` field in each `plugin.json`, the same for every plugin: `0.0.0-dev`, or the release's version in a release build;
   - in an extension, the `marketplace` of each dependency on a CECH plugin, the versioned name of the CECH release the extension builds on;
   - the marketplace catalogue `.claude-plugin/marketplace.json`, named after the namespace (plus the version in a release build), with each plugin's category taken from its folder ([ADR 12](0012-categories-and-names.md));
   - Codex TOML for reviewer agents ([ADR 10](0010-claude-code-and-codex-from-one-source.md));
   - a copy of the root `LICENSE` in each skill folder, byte for byte and with no generated-file line (item 4), so a skill copied without its plugin keeps its licence ([ADR 2](0002-licence-and-contributions.md)); a marketplace with no root `LICENSE` gets none;
   - the code owners file;
   - the catalogue page in `docs/`.
3. One generator writes all of them: `cech build`.
   It works on any marketplace with CECH's layout, so extensions use it too ([ADR 8](0008-a-marketplace-you-extend-not-fork.md)).
4. Generated files are committed, because both harnesses read the catalogue and manifests straight from git.
   Where the format allows comments, a generated file starts with a line saying it is generated and by what.
5. Output is deterministic: plugins sorted by category, then name; fixed formatting; LF line ends and a final newline on every OS.
   Running the build twice changes nothing.
6. Where the build writes a field inside a hand-written file (`plugin.json` `description` or `version`, or a dependency's `marketplace`), it changes only that field and leaves the rest of the file byte for byte as it was.
7. `cech build --check` regenerates in memory and fails on any difference from the committed files.
   It is part of the merge gate.
   A wrong generated file is fixed by changing its source, never by editing the output.

## Consequences

- A description is written once, in `SKILL.md`.
- Every pull request that touches a plugin also commits regenerated files; the gate says which.
- Until `cech build` exists there are no plugins, so nothing needs it.

## Options considered

- **Hand-write every file, plus a lint that compares them:** the lint finds drift but every copy still needs editing by hand.
- **Generate at install or release time, commit nothing:** both harnesses read the catalogue from the repository, so it must be in git.
- **Generate `plugin.json` entirely from `SKILL.md`:** moves author and dependencies into skill frontmatter as fields outside the Agent Skills spec, against [ADR 10](0010-claude-code-and-codex-from-one-source.md).
