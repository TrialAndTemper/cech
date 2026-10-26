# 15. Repository state under .cech/

- Status: accepted
- Date: 2026-10-26
- Deciders: Michał Bożek

## Context

A project that uses CECH commits facts for the `cech` CLI and for plugins: which marketplaces and plugins it enables, later which skill fills each role ([ADR 3](0003-a-marketplace-you-extend-not-fork.md)), and facts a skill reads, such as a tracker's project key.
Extensions add plugins of their own, and their files must not clash with CECH's.
Files scattered over a repository's root are hard to find and can clash with other tools' files.
When several owners write one shared file, an upgrade of one can rewrite the others' parts, and their changes conflict in merges.

Earlier ADRs place repository-wide data in `data/`: `data/harnesses.toml` ([ADR 5](0005-native-tool-features-first.md)) and `data/categories.toml` ([ADR 8](0008-categories-and-names.md)).
Other files have formats set by their harnesses, tools or specs: `SKILL.md` frontmatter is YAML, plugin manifests and Claude Code settings are JSON, Codex settings are TOML, and GitHub workflows are YAML.
YAML reads some unquoted values as other types: `no` can become false and `1.10` a number.
JSON has no comments, which hand-edited files need.
The `cech` CLI is written in Go ([ADR 14](0014-the-cech-cli.md)), whose standard library has no TOML package (checked on 2026-10-21 with Go 1.27.2).

## Decision

1. **One folder.**
   `.cech/` at the root of a project holds the committed files that `cech` and the project's plugins read there.
   Apart from what the project enables ([ADR 3](0003-a-marketplace-you-extend-not-fork.md) item 2), it holds facts only, never a plugin's content.
   It holds no secrets, no values that differ between machines, and no caches; those stay outside the repository or in git-ignored paths outside `.cech/`.
2. **One file per owner**, named after it:
   - `.cech/config.toml` belongs to the `cech` CLI;
   - `.cech/<namespace>/<plugin>.toml` belongs to the plugin of that name, and holds the facts its skill reads.

   The owner defines a file's keys; a person or the owner's tooling edits it.
   A plugin with nothing to keep has no file.
3. **A folder per namespace.**
   Each marketplace's plugins keep their files in the folder named after its namespace: `cech` for CECH, and for an extension the namespace of [ADR 8](0008-categories-and-names.md) item 4.
   So CECH's `cech-prose` keeps `.cech/cech/cech-prose.toml`, and the `acme` extension's `acme-tracker` keeps `.cech/acme/acme-tracker.toml`.
   An extension writes nothing outside its folder.
4. `cech check` fails on, and names, any file under `.cech/` other than `config.toml` and the files of the plugins the project enables, such as one left behind by a plugin the project no longer enables.
5. **TOML.**
   Files in `.cech/` are TOML, like those in `data/`.
   Files whose format a harness, a tool or a spec owns keep it: skill frontmatter, plugin manifests and catalogues, harness settings, workflows, and the eval cases of `claude plugin eval`.
6. `cech` rejects a key it does not know in a file it owns, `.cech/config.toml` or one in `data/`, and names it, so a typo, or a file written by a newer `cech`, fails instead of being ignored.

## Consequences

- A reviewer finds a project's CECH files in one folder, and code owners can be set for it and for each namespace.
- Upgrading or dropping a plugin changes no other plugin's file, and `cech check` fails until a project that stops enabling a plugin removes that plugin's file.
- The CLI's TOML parser, already needed for Codex settings, also reads `.cech/`; it is code of its own or a dependency added to the list in [ADR 14](0014-the-cech-cli.md) item 8.
- Everyone on a project needs a `cech` at least as new as the one that last wrote its config.

## Options considered

- **Files at the repository root** (`cech.toml`, `acme.toml`): clutter, and names can clash with other tools.
- **One shared file for every owner:** merge conflicts, and an upgrade of one owner can rewrite the others' sections.
- **One flat folder for every plugin's file:** extension plugin names carry the namespace too, but a folder per namespace shows at a glance which marketplace owns a file, and lets code owners be set per marketplace.
- **CECH keys inside the harnesses' settings files:** those files belong to the harnesses, which may add a key with the same name later.
- **YAML or JSON for `.cech/`:** YAML's implicit types and JSON's lack of comments, as above.
