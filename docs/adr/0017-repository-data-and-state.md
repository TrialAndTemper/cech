# 17. Repository data and state

- Status: accepted
- Date: 2026-10-26
- Deciders: Michał Bożek

## Context

A marketplace's catalogue and checks need details about it, such as its owner.
A project that uses CECH commits facts for the `cech` CLI and for plugins: which marketplaces and plugins it enables, which skill fills each role ([ADR 8](0008-a-marketplace-you-extend-not-fork.md)), and facts a skill reads, such as a tracker's project key.
Extensions add plugins of their own, and their files must not clash with CECH's.
Files scattered over a repository's root are hard to find and can clash with other tools' files.
When several owners write one shared file, an upgrade of one can rewrite the others' parts, and their changes conflict in merges.

Other files have formats set by their harnesses, tools or specs, such as YAML frontmatter and JSON manifests.
YAML reads some unquoted values as other types: `no` can become false and `1.10` a number.
JSON has no comments, which hand-edited files need.

## Decision

1. **One folder.**
   `.cech/` at the root of a project holds the committed files that `cech` and the project's plugins read there.
   Apart from what the project enables ([ADR 8](0008-a-marketplace-you-extend-not-fork.md) item 2), it holds facts only, never a plugin's content.
   It holds no secrets, no values that differ between machines, and no caches; those stay outside the repository or in git-ignored paths outside `.cech/`.
2. **Files by owner:**
   - `.cech/config.toml` belongs to the `cech` CLI, and so does `.cech/roles.toml`, which it generates from the config and the pinned marketplaces;
   - `.cech/<namespace>/<plugin>.toml` belongs to the plugin of that name, and holds the facts its skill reads.

   The owner defines a file's keys; a person or the owner's tooling edits it.
   A plugin with nothing to keep has no file.
3. **A folder per namespace.**
   Each marketplace's plugins keep their files in the folder named after its namespace: `cech` for CECH, and for an extension the namespace of [ADR 12](0012-categories-and-names.md) item 4.
   So the `acme` extension's `acme-tracker` keeps `.cech/acme/acme-tracker.toml`.
   An extension writes nothing outside its folder.
4. `cech check` fails on, and names, any file under `.cech/` that no owner claims.
   That is any file at the top level other than `config.toml` and `roles.toml`, and any file in the folder of a marketplace that `config.toml` lists, for a plugin the project does not enable.
   It leaves the folders of other namespaces alone.
5. **TOML.**
   Files in `.cech/` are TOML, like those in `data/`.
   Files whose format a harness, a tool or a spec owns keep it, such as plugin manifests and the eval cases of `claude plugin eval`.
6. `cech` rejects, and names, an unknown, misplaced, missing or invalid key in a file it owns, `.cech/config.toml` or one in `data/`, so a typo, or a file written by a newer `cech`, fails instead of being ignored.
7. **Every marketplace describes itself in `data/marketplace.toml`,** CECH included: its `namespace`, `description`, `owner` and code owners `team`, any `categories` it adds, the `roles` it defines, and its own term lists under `terms`.
   An extension's file also has a `cech` key, the one CECH release it builds on ([ADR 8](0008-a-marketplace-you-extend-not-fork.md) item 6), as that release's tag without the `v`.
   A namespace never ends in a version, such as `-1-4-0`, so it cannot look like a release's name.
   A file with a `cech` key is an extension's, and its namespace is not `cech` and does not start with `cech-`.
   A file without one is CECH's own: its namespace is `cech`, and it has no `categories` or `terms`; CECH's categories are in `data/categories.toml` ([ADR 12](0012-categories-and-names.md) item 1).

## Consequences

- A reviewer finds a project's CECH files in one folder, and code owners can be set for it and for each namespace.
- Upgrading or dropping a plugin changes no other plugin's file.
- Everyone on a project needs a `cech` at least as new as the one that last wrote its config.

## Options considered

- **Files at the repository root** (`cech.toml`, `acme.toml`): clutter, and names can clash with other tools.
- **One shared file for every owner:** merge conflicts, and an upgrade of one owner can rewrite the others' sections.
- **One flat folder for every plugin's file:** extension plugin names carry the namespace too, but a folder per namespace shows at a glance which marketplace owns a file, and lets code owners be set per marketplace.
- **CECH keys inside the harnesses' settings files:** those files belong to the harnesses, which may add a key with the same name later.
- **YAML or JSON for `.cech/`:** YAML's implicit types and JSON's lack of comments, as above.
- **An `extension.toml` for extensions only:** CECH's own details would then need another file, or code released with the CLI.
