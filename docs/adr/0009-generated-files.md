# 9. Generated files

- Status: accepted
- Date: 2026-10-17
- Deciders: Michał Bożek

## Context

Claude Code and Codex need some facts about a plugin in several files: the description in `SKILL.md`, in `plugin.json` and in the marketplace catalogue; the category in the folder and in the catalogue; reviewer agents as Markdown and, for Codex, as TOML ([ADR 6](0006-claude-code-and-codex-from-one-source.md)).
Copies kept by hand drift, and a drifted catalogue shows users a wrong description or loads a wrong file.

Checked on 2026-10-17 (Claude Code 2.1.302): `claude plugin validate --strict` fails when `plugin.json` has no `description`, so the field cannot just be left out.

## Decision

1. Per plugin, two files hold hand-written metadata:
   - `SKILL.md`: name, description, `metadata`, and the skill itself;
   - `.claude-plugin/plugin.json`: every field except `description` (name, version, author, licence, dependencies).

   The name appears in both files and as the skill and plugin folder names.
   The validator checks that all of them match, extending the same-name rule of [ADR 7](0007-one-plugin-one-skill.md).

   Repo-wide data in `data/` is hand-written too; it describes the catalogue, not one plugin.
2. Everything else derived from them is generated:
   - the `description` field in each `plugin.json`, copied from `SKILL.md`;
   - the marketplace catalogue `.claude-plugin/marketplace.json`, with each plugin's category taken from its folder ([ADR 8](0008-categories-and-names.md));
   - Codex TOML for reviewer agents ([ADR 6](0006-claude-code-and-codex-from-one-source.md));
   - the code owners file;
   - the catalogue page in `docs/`.
3. One generator writes all of them: `cech build`.
   It works on any marketplace with CECH's layout, so extensions use it too ([ADR 3](0003-a-marketplace-you-extend-not-fork.md)).
4. Generated files are committed, because both harnesses read the catalogue and manifests straight from git.
   Where the format allows comments, a generated file starts with a line saying it is generated and by what.
5. Output is deterministic: plugins sorted by category, then name; fixed formatting; LF line ends and a final newline on every OS.
   Running the build twice changes nothing.
6. Where the build writes a field inside a hand-written file (`plugin.json` `description`), it changes only that field and leaves the rest of the file byte for byte as it was.
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
- **Generate `plugin.json` entirely from `SKILL.md`:** moves version, author and dependencies into skill frontmatter as fields outside the Agent Skills spec, against [ADR 6](0006-claude-code-and-codex-from-one-source.md).
