# 20. The extension model

- Status: accepted
- Date: 2026-10-27
- Deciders: Michał Bożek

## Context

Organisations extend CECH with marketplaces of their own instead of forking it ([ADR 8](0008-a-marketplace-you-extend-not-fork.md)).
An extension needs a layout and a way to build on CECH's skills and tooling.

Probed on 2026-10-27 with Claude Code 2.1.311, local directory marketplaces and project-scope installs:

- A dependency on another marketplace's plugin must name that marketplace exactly: an `acme` plugin that depends on `cech-prose` loads when the dependency names `cech-0-1-0`, and fails when it names `cech` or no marketplace, since Claude Code then looks in `acme`.

Probed on 2026-10-28 with Codex CLI 0.165.0: it clones HTTPS and SSH marketplaces with the system `git`, so the machine's git credentials work.

The [Claude Code plugin dependency docs](https://code.claude.com/docs/en/plugins/dependencies), read on 2026-10-27, match version ranges against `<plugin>--v<version>` tags.
The [docs on private marketplaces](https://code.claude.com/docs/en/plugins/host-marketplace), read the same day, say git uses the machine's credentials and no catalogue field holds a token.

## Decision

1. **An extension is a marketplace repository with CECH's layout:** plugins in `plugins/<category>/<name>/`, data in `data/`, and the files `cech build` generates ([ADR 18](0018-generated-files.md)).
   Its catalogue lists only its own plugins.
   An extension uses a CECH plugin only by depending on it (item 3), never by listing or copying it.
2. **Replace, do not patch.**
   To replace a CECH skill, a project enables the extension's plugin instead.
   The replacement can fill the same role ([ADR 8](0008-a-marketplace-you-extend-not-fork.md) item 3), but a plugin that depends on the CECH plugin by name still needs the CECH plugin.
   An extension never ships edited copies of CECH text.
3. **Build on a CECH plugin by depending on it.**
   An extension plugin depends on a CECH plugin by name, such as `{"name": "cech-prose"}` ([ADR 11](0011-one-plugin-one-skill.md) item 3).
   A name starting with `cech-` marks a CECH plugin.
   `cech build` sets each CECH dependency's marketplace from the `cech` key ([ADR 17](0017-repository-data-and-state.md) item 7), `cech-0-1-0` for `cech = "0.1.0"`, replacing any value there ([ADR 18](0018-generated-files.md) items 1 and 2).
   `cech validate` rejects a CECH dependency written as a plain string, which has no marketplace to set, or with a `version`, since CECH has no per-plugin tags to match.
   Dependencies on the extension's own plugins name no marketplace, so they still work when a release renames the catalogue.
4. **One CECH release per extension release.**
   Some extension plugins depend on CECH, directly or through other plugins.
   A project that pins a different CECH release cannot enable them, because their dependencies name a release it lacks: `cech enable` refuses them, and `cech check` fails ([ADR 19](0019-per-project-plugin-enablement.md) items 5 and 8).
5. **The same tooling.**
   An extension's CI runs `cech build --check` and `cech validate` ([ADR 4](0004-gates-and-ci.md) item 5), and `cech check` ([ADR 3](0003-vendor-neutrality.md) item 6).
   The rules and CECH's categories come from the CLI version that runs, not from the release the `cech` key names.
   In an extension, `cech validate` also rejects a plugin name without the namespace prefix, a category that is neither CECH's nor its own, and a dependency item 3 does not allow.
   `cech check` scans an extension with CECH's generic term rules, the extension's own lists and its private list ([ADR 3](0003-vendor-neutrality.md) item 4).
   An extension that also has a `.cech/config.toml` gets a project's checks too.
6. **Private repositories use the machine's git credentials.**
   No committed file holds a token, as for `.cech/config.toml` ([ADR 19](0019-per-project-plugin-enablement.md) item 1); the generic term rules catch tokens.

## Consequences

- To move to a new CECH release, an extension changes one value and releases; projects then move their CECH and extension pins together.
- A project can move to a new CECH release only after every extension it uses with CECH dependencies has a release built on it ([ADR 8](0008-a-marketplace-you-extend-not-fork.md)).
- If CECH adds a category that an extension also adds, the extension's `cech validate` fails until it renames its own.

## Options considered

- **A fork per organisation:** every upgrade is a merge.
- **An overlay engine that patches CECH text:** hidden changes, and a second format to maintain.
- **A range of supported CECH releases:** a dependency resolves only in the one marketplace it names, so a range works only for extensions with no CECH dependency.
- **Writing versioned dependency names by hand:** each upgrade edits every manifest, and a missed one breaks a plugin.
- **Generating `allowCrossMarketplaceDependenciesOn`:** it controls Claude Code's own installs, which were not probed; `cech enable` enables dependencies instead.
- **Dependencies on other extensions:** `cech build` cannot know their release names, so each would break at that extension's next release.
