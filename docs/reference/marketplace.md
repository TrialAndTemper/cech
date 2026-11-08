# A marketplace

What `cech build` reads in a marketplace, CECH or an extension, and the rules it holds the marketplace to ([ADR 12](../adr/0012-categories-and-names.md)).

## Layout

Each plugin lives in `plugins/<category>/<name>/`, with its skill in `skills/<name>/SKILL.md` and its manifest in `.claude-plugin/plugin.json`.
The plugin folder, the skill folder, the manifest's `name` and the skill's `name` are the same.
`cech build` rejects, and names, a manifest or skill `name` that differs from its folder, and a plugin whose name another plugin already has, in any category.

## The data file

`data/marketplace.toml` describes the marketplace ([ADR 17](../adr/0017-repository-data-and-state.md) item 7).

| Key | Required | Holds |
| --- | --- | --- |
| `namespace` | yes | The catalogue's name outside a release, and the prefix of every plugin name in an extension; a name that does not end in a version such as `-1-4-0` or `-1-4-0-rc-1` |
| `description` | yes | The catalogue's description |
| `owner.name`, `owner.email` | `name` only | The catalogue's owner |
| `team` | no | The maintainer team, as `@org/team`, which owns every path the code owners file gives no plugin line; without it there is no code owners file |
| `cech` | in an extension only | The CECH release the extension builds on, as `X.Y.Z` or `X.Y.Z-rc.N` |
| `categories` | no; an extension only | The categories it adds: `[[categories]]` tables with `name` and `holds` |
| `roles` | no | The roles it defines: `[[roles]]` tables with `name`, `means` and an optional `many`, a bool |
| `terms.rules`, `terms.names` | no; an extension only | Paths from the repository root to its own term rule and name files |

- A file without `cech` is CECH's own: its namespace is `cech`, and it has no `categories` or `terms`.
- An extension's namespace is not `cech` and does not start with `cech-`.
- Each role has a `name` following the name rules, not starting with `cech-`, and in an extension starting with `<namespace>-`; no role is defined twice, and each has a `means`.
- The build rejects, and names, an unknown, misplaced, missing or invalid key.

## Names

A plugin, skill, category, role or namespace name:

- uses only `a-z`, `0-9` and `-`;
- starts with a letter, does not end with `-`, and has no `--`;
- has at most 64 characters.

Such a name is valid in both harnesses and in Agent Skills.
`cech build` rejects, and names, a category or plugin folder whose name breaks these rules.

## Categories

CECH's categories are built into the `cech` CLI, from `cli/internal/marketplace/categories.toml`:

| Category | Holds |
| --- | --- |
| `practice` | Tool-neutral engineering rules (testing, coding, config, logging, dependencies, containers, infrastructure as code) |
| `writing` | Prose, docs, decision records, changelogs, commit messages |
| `agent` | How agents work in a repository (instruction files, task records, estimates, change specs) |
| `workflow` | Multi-step flows that move a change through stages |
| `lang` | Languages and runtimes |
| `framework` | Libraries and frameworks |
| `tool` | One command-line tool each |
| `platform` | Hosted services: forges, trackers, clouds, observability |
| `meta` | Authoring and testing CECH skills |

Adding a category is a reviewed change to that file.
Removing or merging one is a breaking change of the CLI, since an extension or a plugin may use it.

An extension may add categories of its own in its data file; each has a `name` following the name rules, not one of CECH's, and a `holds`.

## Release builds

`cech build --release V` builds a catalogue release ([ADR 14](../adr/0014-releases-and-versions.md)).

- `V` is `X.Y.Z` or `X.Y.Z-rc.N`, with no `v`: each part decimal with no leading zeros, and `N` from 1.
- The catalogue is named `<namespace>-V` with each `.` turned into `-`, such as `cech-0-2-0-rc-1`, and has no `version`.
- Every `plugin.json` gets `V` as its `version`.
- An extension's final release must build on a final CECH release: with a `V` that has no `-rc.N`, the build fails when the `cech` key names a pre-release.

Without `--release`, the catalogue is named after the namespace and every `plugin.json` has the `version` `0.0.0-dev`.

## A private extension

A private extension stays on authenticated git ([ADR 22](../adr/0022-two-install-paths.md) item 8).
For copies made with a skill installer from a private source:

- do not share them through a skills.sh pack: its docs call a pack "unlisted, not access-controlled", so anyone with its link can install it;
- set `DISABLE_TELEMETRY=1` for every `npx skills` command, which otherwise can send source and skill names (checked in `skills` 1.7.4).
