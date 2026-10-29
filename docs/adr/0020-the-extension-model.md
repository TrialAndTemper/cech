# 20. The extension model

- Status: accepted
- Date: 2026-10-27
- Deciders: Michał Bożek

## Context

Organisations extend CECH with marketplaces of their own instead of forking it ([ADR 8](0008-a-marketplace-you-extend-not-fork.md)).
Earlier ADRs settle how extensions name their plugins ([ADR 12](0012-categories-and-names.md) items 4 and 5) and how a project pins releases under versioned catalogue names ([ADR 19](0019-per-project-plugin-enablement.md)).
Still open: what an extension repository holds, how it builds on CECH's skills and tooling, and where `cech build` finds a catalogue's details ([ADR 18](0018-generated-files.md)).

Probed on 2026-10-27 with Claude Code 2.1.311, local directory marketplaces and project-scope installs:

- A dependency on another marketplace's plugin must name that marketplace exactly.
  An `acme` plugin that depends on `cech-prose` loads when the dependency names `cech-0-1-0`.
  Naming `cech` fails; naming no marketplace fails too, because Claude Code then looks in `acme`.

Probed on 2026-10-28 with Codex CLI 0.165.0:

- It clones HTTPS and SSH marketplaces with the system `git`, so the machine's git credentials work.
- It does not resolve dependencies: it loads a plugin whose dependency is missing ([ADR 10](0010-claude-code-and-codex-from-one-source.md)).

The [Claude Code plugin dependency docs](https://code.claude.com/docs/en/plugins/dependencies), read on 2026-10-27, match version ranges against `<plugin>--v<version>` tags.
The [docs on private marketplaces](https://code.claude.com/docs/en/plugins/host-marketplace), read the same day, say git uses the machine's credentials and no catalogue field holds a token.

## Decision

1. **An extension is a marketplace repository with CECH's layout:** plugins in `plugins/<category>/<name>/`, data in `data/`, and the files `cech build` generates ([ADR 18](0018-generated-files.md)).
   Its catalogue lists only its own plugins.
   An extension uses a CECH plugin only by depending on it (item 4), never by listing or copying it.
2. **Every marketplace describes itself in `data/marketplace.toml`,** CECH included:

   ```toml
   namespace = "acme"
   description = "Acme's agent skills, built on CECH"
   cech = "0.1.0"
   team = "@acme/platform"

   [owner]
   name = "Acme platform team"
   email = "platform@acme.example"

   [[categories]]
   name = "brand"
   holds = "Brand voice and visual identity"

   [terms]
   rules = ["checks/terms/rules.toml"]
   names = ["checks/terms/names.txt"]
   ```

   | Key | Required | Meaning and rules |
   | --- | --- | --- |
   | `namespace` | yes | The catalogue's base name, to which a release adds the version ([ADR 19](0019-per-project-plugin-enablement.md) items 3 and 6), and the prefix of every plugin name; follows [ADR 12](0012-categories-and-names.md) item 3's name rules and does not end in a version such as `-1-4-0` or `-1-4-0-rc-2` |
   | `description` | yes | The catalogue's description; not empty |
   | `cech` | in an extension | The one CECH release the extension builds on, as `X.Y.Z` or `X.Y.Z-rc.N`: its tag without the `v` ([ADR 19](0019-per-project-plugin-enablement.md) item 1) |
   | `owner` | yes | The catalogue's owner: a `name`, not empty, and optionally an `email` |
   | `team` | no | The catch-all code owners team, `@<org>/<team>` ([ADR 13](0013-attribution.md) item 4); with none, no code owners file is built |
   | `categories` | no; extensions only | Categories added to CECH's ([ADR 12](0012-categories-and-names.md) item 6): each a unique `name` that follows the plugin name rules and is not one of CECH's, and a `holds` text, not empty |
   | `roles` | no | Roles the marketplace defines: each a unique `name` with the characters and length of plugin names that does not start with `cech-` and, in an extension, starts with `<namespace>-`; a `means` text of one line, not empty; and optionally `many`, a boolean, true when a project may bind several skills to the role |
   | `terms.rules`, `terms.names` | no; extensions only | Paths from the repository root to existing files inside it, in the formats of CECH's generic and vendor-name term rules; the term check skips these files |

   A `cech` key makes the file an extension's; its namespace is not `cech` and does not start with `cech-`.
   A file without one is CECH's own: its namespace is `cech`, and it has no `categories` or `terms`.
   The CLI rejects a file with a missing, unknown, misplaced or invalid key, and names the key ([ADR 17](0017-repository-data-and-state.md) item 6).
3. **Replace, do not patch.**
   To replace a CECH skill, a project enables the extension's plugin instead.
   The replacement can fill the same role ([ADR 8](0008-a-marketplace-you-extend-not-fork.md) item 3), but a plugin that depends on the CECH plugin by name still needs the CECH plugin.
   An extension never ships edited copies of CECH text.
4. **Build on a CECH plugin by depending on it.**
   An extension plugin depends on a CECH plugin by name, such as `{"name": "cech-prose"}` ([ADR 11](0011-one-plugin-one-skill.md) item 3).
   A name starting with `cech-` marks a CECH plugin.
   `cech build` sets each CECH dependency's marketplace from the `cech` key, `cech-0-1-0` for `cech = "0.1.0"`, replacing any value there ([ADR 18](0018-generated-files.md) items 1 and 2).
   `cech validate` rejects a CECH dependency written as a plain string, which has no marketplace to set, or with a `version`, since CECH has no per-plugin tags to match.
   It also checks that each named CECH plugin exists in that release's catalogue in CECH's public repository.
   Dependencies on the extension's own plugins name no marketplace, so they still work when a release renames the catalogue.
5. **One CECH release per extension release.**
   A project that pins a different CECH release cannot enable the extension's plugins that depend on CECH, directly or through the extension's plugins.
   Their dependencies name a CECH release the project does not have.
   `cech enable` refuses them, and `cech check` fails ([ADR 19](0019-per-project-plugin-enablement.md) items 5 and 8).
6. **The same tooling.**
   An extension's CI runs `cech build --check` and `cech validate` ([ADR 4](0004-gates-and-ci.md) item 5), and `cech check` ([ADR 3](0003-vendor-neutrality.md) item 6).
   The rules and CECH's categories come from the CLI version that runs, not from the release the `cech` key names.
   In an extension, `cech validate` also rejects a plugin name without the namespace prefix, a category that is neither CECH's nor its own, and any dependency item 4 does not allow.
   `cech check` scans an extension with CECH's generic term rules, the extension's own lists and its private list ([ADR 3](0003-vendor-neutrality.md) item 4).
   The rules' email allowlist leaves out CECH's own addresses; CECH's vendor names and commit-message tracker-key rule do not apply.
   An extension that also has a `.cech/config.toml` gets a project's checks too.
7. **Private repositories use the machine's git credentials.**
   No committed file holds a token, as for `.cech/config.toml` ([ADR 19](0019-per-project-plugin-enablement.md) item 1); the generic term rules catch tokens.

## Consequences

- To move to a new CECH release, an extension changes one value and releases; projects then move their CECH and extension pins together.
- A project can move to a new CECH release only after every extension it uses with CECH dependencies has a release built on it ([ADR 8](0008-a-marketplace-you-extend-not-fork.md)).
- `cech build` reads the catalogue's name, description, owner and code owners team from `data/marketplace.toml`, so CECH adds that file when the build code lands.
  It names the catalogue after the namespace, and a release build adds the version.
- If CECH adds a category that an extension also adds, the extension's `cech validate` fails until it renames its own.
- The probed behaviour goes into the harness data file when code relies on it ([ADR 9](0009-native-harness-features-first.md) item 3).

## Options considered

- **A fork per organisation:** every upgrade is a merge.
- **An overlay engine that patches CECH text:** hidden changes, and a second format to maintain.
- **A range of supported CECH releases:** a dependency resolves only in the one marketplace it names, so a range works only for extensions with no CECH dependency.
- **Writing versioned dependency names by hand:** each upgrade edits every manifest, and a missed one breaks a plugin.
- **An `extension.toml` for extensions only:** CECH's own details would then need another file, or code released with the CLI.
- **Generating `allowCrossMarketplaceDependenciesOn`:** it controls Claude Code's own installs, which were not probed; `cech enable` enables dependencies instead.
- **Dependencies on other extensions:** `cech build` cannot know their release names, so each would break at that extension's next release.
