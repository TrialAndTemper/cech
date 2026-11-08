# A marketplace

What `cech build` reads in a marketplace, CECH or an extension, and the rules it holds the marketplace to ([ADR 12](../adr/0012-categories-and-names.md)).

## Layout

Each plugin lives in `plugins/<category>/<name>/`, with its skill in `skills/<name>/SKILL.md` and its manifest in `.claude-plugin/plugin.json`.
The plugin folder, the skill folder, the manifest's `name` and the skill's `name` are the same.
`cech build` rejects, and names, a manifest or skill `name` that differs from its folder, and a plugin whose name another plugin already has, in any category.

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
