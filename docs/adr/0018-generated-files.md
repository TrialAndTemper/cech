# 18. Generated files

- Status: accepted
- Date: 2026-10-17
- Deciders: Michał Bożek

## Context

Claude Code and Codex need some facts about a plugin in several files: the description in `SKILL.md`, in `plugin.json` and in the marketplace catalogue; the category in the folder and in the catalogue; reviewer agents as Markdown and, for Codex, as TOML ([ADR 10](0010-claude-code-and-codex-from-one-source.md)).
Copies kept by hand drift, and a drifted catalogue shows users a wrong description or loads a wrong file.

Checked on 2026-10-17 (Claude Code 2.1.302): `claude plugin validate --strict` fails when `plugin.json` has no `description`, so the field cannot just be left out.
Checked again on 2026-10-28 (Claude Code 2.1.312): it also fails when `plugin.json` has no `version`, and Claude Code uses `version` to decide whether an update changes anything.
The same check fails when the marketplace catalogue has no `description`, and accepts one at the top level.

## Decision

1. Per plugin, only `SKILL.md` and `plugin.json` hold hand-written metadata, so a description is written once, in `SKILL.md`.
   The name appears in both files and as the skill and plugin folder names, and all of them must match, extending the same-name rule of [ADR 11](0011-one-plugin-one-skill.md).
   Repo-wide data in `data/` is hand-written too ([ADR 17](0017-repository-data-and-state.md)); it describes the catalogue, not one plugin.
2. Everything derived from them is generated: the derived fields of each `plugin.json`, such as its description and version ([ADR 14](0014-releases-and-versions.md) item 3), the marketplace catalogue, Codex TOML for reviewer agents ([ADR 10](0010-claude-code-and-codex-from-one-source.md)), the code owners file ([ADR 13](0013-attribution.md) item 4) and the catalogue page.
   Each skill folder also gets a copy of the root `LICENSE`, so a skill copied without its plugin keeps its licence ([ADR 2](0002-licence-and-contributions.md)).
3. One generator writes all of them: `cech build`.
   It works on any marketplace with CECH's layout, so extensions use it too ([ADR 8](0008-a-marketplace-you-extend-not-fork.md)).
4. Generated files are committed, because both harnesses read the catalogue and manifests straight from git.
   Where the format allows comments, a generated file starts with a line saying it is generated and by what.
5. Output is deterministic and byte for byte the same on every OS; running the build twice changes nothing.
6. Where the build writes a field inside a hand-written file, it changes only that field and leaves the rest of the file byte for byte as it was.
7. A check mode regenerates in memory and fails on any difference from the committed files, and is a gate.
   A wrong generated file is fixed by changing its source, never by editing the output.

## Consequences

- A description is written once, in `SKILL.md`.
- Every pull request that touches a plugin also commits regenerated files; the gate says which.

## Options considered

- **Hand-write every file, plus a lint that compares them:** the lint finds drift but every copy still needs editing by hand.
- **Generate at install or release time, commit nothing:** both harnesses read the catalogue from the repository, so it must be in git.
- **Generate `plugin.json` entirely from `SKILL.md`:** moves author and dependencies into skill frontmatter as fields outside the Agent Skills spec, against [ADR 10](0010-claude-code-and-codex-from-one-source.md).
